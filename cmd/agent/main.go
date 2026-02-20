package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/attribution"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/otel"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/safety"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/signals"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/toolkitcfg"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/webhook"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var version = "dev"

const (
	schemaPathSLO   = "docs/contracts/v1/slo-event.schema.json"
	schemaPathProbe = "docs/contracts/v1alpha1/probe-event.schema.json"
)

type eventKindMode int

const (
	eventKindSLO eventKindMode = iota + 1
	eventKindProbe
	eventKindBoth
)

func parseEventKind(value string) (eventKindMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "slo":
		return eventKindSLO, nil
	case "probe":
		return eventKindProbe, nil
	case "both", "all":
		return eventKindBoth, nil
	default:
		return 0, fmt.Errorf("unsupported event kind %q (expected slo|probe|both)", value)
	}
}

func (k eventKindMode) includesSLO() bool {
	return k == eventKindSLO || k == eventKindBoth
}

func (k eventKindMode) includesProbe() bool {
	return k == eventKindProbe || k == eventKindBoth
}

type outputWriters struct {
	mode          string
	sloExporter   *otel.SLOEventExporter
	probeExporter *otel.ProbeEventExporter
	encoder       *json.Encoder
	file          *os.File
	mu            sync.Mutex
}

func newOutputWriters(mode string, path string, endpoint string, timeout time.Duration) (*outputWriters, error) {
	w := &outputWriters{mode: mode}
	switch mode {
	case "stdout":
		w.encoder = json.NewEncoder(os.Stdout)
		return w, nil
	case "jsonl":
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		f, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		w.file = f
		w.encoder = json.NewEncoder(f)
		return w, nil
	case "otlp":
		w.sloExporter = otel.NewSLOEventExporter(endpoint, "llm-slo-ebpf-toolkit", "llm-slo-ebpf-toolkit/agent", timeout)
		w.probeExporter = otel.NewProbeEventExporter(endpoint, "llm-slo-ebpf-toolkit", "llm-slo-ebpf-toolkit/agent", timeout)
		return w, nil
	default:
		return nil, fmt.Errorf("unsupported output mode %q", mode)
	}
}

func (w *outputWriters) EmitSLO(ev schema.SLOEvent) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.mode == "otlp" {
		return w.sloExporter.ExportBatch([]schema.SLOEvent{ev})
	}
	return w.encoder.Encode(map[string]any{
		"kind":    "slo",
		"payload": ev,
	})
}

func (w *outputWriters) EmitProbe(ev schema.ProbeEventV1) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.mode == "otlp" {
		return w.probeExporter.ExportBatch([]schema.ProbeEventV1{ev})
	}
	return w.encoder.Encode(map[string]any{
		"kind":    "probe",
		"payload": ev,
	})
}

func (w *outputWriters) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		_ = w.file.Close()
	}
}

type agentMetrics struct {
	registry *prometheus.Registry

	heartbeat      prometheus.Gauge
	up             prometheus.Gauge
	cpuOverheadPct prometheus.Gauge

	eventKindGauge      *prometheus.GaugeVec
	capabilityModeGauge *prometheus.GaugeVec
	signalEnabledGauge  *prometheus.GaugeVec
	droppedEvents       *prometheus.CounterVec

	helloSyscalls *prometheus.CounterVec
	dnsLatency    *prometheus.HistogramVec
	probeEvents   *prometheus.CounterVec
}

func newAgentMetrics(eventKind string, capabilityMode string, supportedSignals []string, enabledSignals []string) *agentMetrics {
	registry := prometheus.NewRegistry()
	m := &agentMetrics{
		registry: registry,
		heartbeat: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_slo_agent_heartbeat",
			Help: "Unix timestamp of latest emitted sample.",
		}),
		up: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_slo_agent_up",
			Help: "Agent process liveness.",
		}),
		cpuOverheadPct: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "llm_slo_agent_cpu_overhead_pct",
			Help: "Estimated agent CPU overhead percentage.",
		}),
		eventKindGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "llm_slo_agent_event_kind",
			Help: "Selected event-kind mode (one-hot gauge).",
		}, []string{"kind"}),
		capabilityModeGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "llm_slo_agent_capability_mode",
			Help: "Detected capability mode (one-hot gauge).",
		}, []string{"mode"}),
		signalEnabledGauge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "llm_slo_agent_signal_enabled",
			Help: "Signal enablement toggle by signal name.",
		}, []string{"signal"}),
		droppedEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_slo_agent_dropped_events_total",
			Help: "Dropped probe events by reason.",
		}, []string{"reason"}),
		helloSyscalls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_ebpf_hello_syscalls_total",
			Help: "Hello tracer syscall events by comm.",
		}, []string{"node", "pod", "comm"}),
		dnsLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "llm_ebpf_dns_latency_ms",
			Help:    "DNS latency observed from probe events.",
			Buckets: []float64{1, 2, 5, 10, 20, 40, 80, 120, 200, 400, 800},
		}, []string{"node", "pod", "namespace"}),
		probeEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_ebpf_probe_events_total",
			Help: "Probe events observed by signal and status.",
		}, []string{"signal", "status"}),
	}

	registry.MustRegister(
		m.heartbeat,
		m.up,
		m.cpuOverheadPct,
		m.eventKindGauge,
		m.capabilityModeGauge,
		m.signalEnabledGauge,
		m.droppedEvents,
		m.helloSyscalls,
		m.dnsLatency,
		m.probeEvents,
	)

	m.up.Set(1)
	m.heartbeat.Set(float64(time.Now().UTC().Unix()))
	m.cpuOverheadPct.Set(0)

	for _, kind := range []string{"slo", "probe", "both"} {
		v := 0.0
		if kind == eventKind {
			v = 1
		}
		m.eventKindGauge.WithLabelValues(kind).Set(v)
	}

	for _, mode := range []string{string(signals.CapabilityCoreFull), string(signals.CapabilityBCCDegraded)} {
		v := 0.0
		if mode == capabilityMode {
			v = 1
		}
		m.capabilityModeGauge.WithLabelValues(mode).Set(v)
	}

	sort.Strings(supportedSignals)
	enabledSet := make(map[string]struct{}, len(enabledSignals))
	for _, signal := range enabledSignals {
		enabledSet[signal] = struct{}{}
	}
	for _, signal := range supportedSignals {
		_, enabled := enabledSet[signal]
		if enabled {
			m.signalEnabledGauge.WithLabelValues(signal).Set(1)
		} else {
			m.signalEnabledGauge.WithLabelValues(signal).Set(0)
		}
	}

	return m
}

func (m *agentMetrics) SetHeartbeat(ts time.Time) {
	m.heartbeat.Set(float64(ts.UTC().Unix()))
}

func (m *agentMetrics) SetCPUOverhead(pct float64) {
	if pct < 0 {
		pct = 0
	}
	m.cpuOverheadPct.Set(pct)
}

func (m *agentMetrics) SetEnabledSignals(supported []string, enabled []string) {
	enabledSet := make(map[string]struct{}, len(enabled))
	for _, signal := range enabled {
		enabledSet[signal] = struct{}{}
	}
	for _, signal := range supported {
		if _, ok := enabledSet[signal]; ok {
			m.signalEnabledGauge.WithLabelValues(signal).Set(1)
		} else {
			m.signalEnabledGauge.WithLabelValues(signal).Set(0)
		}
	}
}

func (m *agentMetrics) ObserveProbeEvent(ev schema.ProbeEventV1, enableRealProbeMetrics bool) {
	m.probeEvents.WithLabelValues(ev.Signal, ev.Status).Inc()
	if !enableRealProbeMetrics {
		return
	}
	if ev.Signal == signals.SignalDNSLatencyMS {
		m.dnsLatency.WithLabelValues(nonEmpty(ev.Node, "unknown-node"), nonEmpty(ev.Pod, "unknown-pod"), nonEmpty(ev.Namespace, "default")).Observe(ev.Value)
	}
}

func (m *agentMetrics) IncDropped(reason string) {
	m.droppedEvents.WithLabelValues(reason).Inc()
}

func (m *agentMetrics) IncHello(node string, pod string, comm string, count uint64) {
	if count == 0 {
		return
	}
	m.helloSyscalls.WithLabelValues(nonEmpty(node, "unknown-node"), nonEmpty(pod, "unknown-pod"), nonEmpty(comm, "unknown")).Add(float64(count))
}

func nonEmpty(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return strings.TrimSpace(v)
}

func startMetricsServer(bind string, metrics *agentMetrics) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	server := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("metrics server failed: %v", err)
		}
	}()
}

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	var (
		cluster   = flag.String("cluster", "local", "cluster name")
		namespace = flag.String("namespace", "default", "namespace")
		workload  = flag.String("workload", "llm-slo-agent", "workload")
		service   = flag.String("service", "agent", "service")
		node      = flag.String("k8s-node", "unknown-node", "node label")
		pod       = flag.String("pod", "llm-slo-agent", "pod name")
		container = flag.String("container", "agent", "container name")

		scenario   = flag.String("scenario", "baseline", "synthetic scenario name")
		count      = flag.Int("count", 0, "sample count (0 = stream mode)")
		intervalMS = flag.Int("interval-ms", 1000, "emit interval for stream mode")

		eventKind = flag.String("event-kind", "probe", "event kind: slo|probe|both")

		outputMode   = flag.String("output", "stdout", "output mode: stdout|jsonl|otlp")
		outputPath   = flag.String("output-path", "artifacts/agent/events.jsonl", "output file when output=jsonl")
		otlpEndpoint = flag.String(
			"otlp-endpoint",
			"http://otel-collector.observability.svc.cluster.local:4318/v1/logs",
			"OTLP/HTTP logs endpoint when output=otlp",
		)
		otlpTimeoutMS = flag.Int("otlp-timeout-ms", 5000, "OTLP export timeout in milliseconds")

		webhookURL       = flag.String("webhook-url", "", "webhook endpoint URL (empty = disabled)")
		webhookSecret    = flag.String("webhook-secret", "", "HMAC-SHA256 secret for webhook signing")
		webhookFormat    = flag.String("webhook-format", "generic", "webhook payload format: generic|pagerduty|opsgenie")
		webhookTimeoutMS = flag.Int("webhook-timeout-ms", 5000, "webhook HTTP timeout in milliseconds")

		capabilityMode      = flag.String("capability-mode", "auto", "capability mode: auto|core_full|bcc_degraded")
		disableSignals      = flag.String("disable-signals", "", "comma-separated signal names to disable")
		disableOverhead     = flag.Bool("disable-overhead-guard", false, "disable overhead guard")
		configPath          = flag.String("config", filepath.Join("config", "toolkit.yaml"), "toolkit config path")
		enableHelloTracer   = flag.Bool("enable-hello-tracer", false, "enable hello tracer metric path")
		helloTargetComm     = flag.String("hello-target-comm", "rag-service,llama-server", "comma-separated comm names for hello tracer")
		enableRealProbeMets = flag.Bool("enable-real-probe-metrics", true, "enable probe-derived metrics on /metrics")

		metricsBind = flag.String("metrics-bind", ":2112", "metrics and health bind address")
		probeSmoke  = flag.Bool("probe-smoke", false, "run eBPF smoke check and exit")
	)
	flag.Parse()

	if *probeSmoke {
		if err := collector.ProbeSmokeCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "probe smoke failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("probe smoke ok")
		return
	}

	kindMode, err := parseEventKind(*eventKind)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	cfg := toolkitcfg.Default()
	if *configPath != "" {
		loaded, loadErr := toolkitcfg.Load(*configPath)
		if loadErr != nil {
			log.Printf("config load warning (%s): %v; using defaults", *configPath, loadErr)
		} else {
			cfg = loaded
		}
	}

	mode := signals.ParseCapabilityMode(*capabilityMode)
	supportedSignals := signals.SupportedSignalsForMode(mode)
	enabledSignalSet := chooseEnabledSignals(cfg.SignalSet, parseCSV(*disableSignals), supportedSignals)

	enricher := signals.ProcMetadataEnricher{
		Next: signals.StaticMetadataEnricher{Defaults: signals.Metadata{
			Node:      *node,
			Namespace: *namespace,
			Pod:       *pod,
			Container: *container,
			Service:   *service,
			Workload:  *workload,
			PID:       os.Getpid(),
			TID:       os.Getpid(),
		}},
	}
	generator := signals.NewGenerator(mode, enabledSignalSet, enricher)

	writers, err := newOutputWriters(*outputMode, *outputPath, *otlpEndpoint, time.Duration(*otlpTimeoutMS)*time.Millisecond)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open output failed: %v\n", err)
		os.Exit(1)
	}
	defer writers.Close()

	// Resolve webhook config: CLI flags override config file values.
	whURL := *webhookURL
	whSecret := *webhookSecret
	whFormat := *webhookFormat
	whTimeout := *webhookTimeoutMS
	if whURL == "" && cfg.Webhook.Enabled && cfg.Webhook.URL != "" {
		whURL = cfg.Webhook.URL
		whSecret = cfg.Webhook.Secret
		if cfg.Webhook.Format != "" {
			whFormat = cfg.Webhook.Format
		}
		if cfg.Webhook.TimeoutMS > 0 {
			whTimeout = cfg.Webhook.TimeoutMS
		}
	}

	var webhookExporter *webhook.Exporter
	if whURL != "" {
		webhookExporter = webhook.New(whURL, whSecret, webhook.Format(whFormat), whTimeout)
		log.Printf("webhook exporter enabled: %s (format=%s)", whURL, whFormat)
	}

	var bayesAttributor *attribution.BayesianAttributor
	if webhookExporter != nil {
		bayesAttributor = attribution.NewBayesianAttributor()
	}

	metrics := newAgentMetrics(*eventKind, string(mode), supportedSignals, generator.EnabledSignals())
	startMetricsServer(*metricsBind, metrics)

	if *intervalMS <= 0 {
		fmt.Fprintln(os.Stderr, "interval-ms must be > 0")
		os.Exit(1)
	}

	runtimeLimiter := safety.NewRateLimiter(cfg.Sampling.EventsPerSecondLimit)
	var guard *safety.OverheadGuard
	if !*disableOverhead && runtime.GOOS == "linux" {
		guard = safety.NewOverheadGuard(cfg.Safety.MaxOverheadPct)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if *enableHelloTracer {
		targetComms := parseCSV(*helloTargetComm)
		helloTracer := collector.NewHelloTracer(targetComms, 2*time.Second)
		go helloTracer.Start(ctx, func(ev collector.HelloEvent) {
			metrics.IncHello(*node, *pod, ev.Comm, ev.Count)
			if !kindMode.includesProbe() {
				return
			}
			probeEvent := schema.ProbeEventV1{
				TSUnixNano: ev.Timestamp.UnixNano(),
				Signal:     "hello_sys_enter_write_total",
				Node:       *node,
				Namespace:  *namespace,
				Pod:        *pod,
				Container:  *container,
				PID:        os.Getpid(),
				TID:        os.Getpid(),
				Value:      float64(ev.Count),
				Unit:       "count",
				Status:     "ok",
			}
			if !runtimeLimiter.Allow(ev.Timestamp) {
				metrics.IncDropped("rate_limit")
				return
			}
			if err := schema.ValidateAgainstSchema(schemaPathProbe, probeEvent); err != nil {
				metrics.IncDropped("schema")
				log.Printf("hello tracer probe event dropped: %v", err)
				return
			}
			if err := writers.EmitProbe(probeEvent); err != nil {
				metrics.IncDropped("emit")
				log.Printf("hello tracer probe emit failed: %v", err)
			}
		})
	}

	meta := collector.SampleMeta{
		Cluster:   *cluster,
		Namespace: *namespace,
		Workload:  *workload,
		Service:   *service,
		Node:      *node,
	}

	emitOne := func(idx int, now time.Time) error {
		sample, err := collector.BuildSyntheticSample(*scenario, idx, now.UTC(), meta)
		if err != nil {
			return err
		}

		if kindMode.includesSLO() {
			sloEvents := collector.NormalizeSample(sample)
			for _, event := range sloEvents {
				if err := schema.ValidateAgainstSchema(schemaPathSLO, event); err != nil {
					metrics.IncDropped("schema")
					return err
				}
				if err := writers.EmitSLO(event); err != nil {
					metrics.IncDropped("emit")
					return err
				}
			}
		}

		probeMeta := signals.Metadata{
			Node:      *node,
			Namespace: *namespace,
			Pod:       *pod,
			Container: *container,
			Service:   *service,
			Workload:  *workload,
			PID:       os.Getpid(),
			TID:       os.Getpid(),
			TraceID:   sample.TraceID,
		}
		probeEvents := generator.Generate(sample, probeMeta)
		for _, event := range probeEvents {
			metrics.ObserveProbeEvent(event, *enableRealProbeMets)
			if !kindMode.includesProbe() {
				continue
			}
			if !runtimeLimiter.Allow(now) {
				metrics.IncDropped("rate_limit")
				continue
			}
			if err := schema.ValidateAgainstSchema(schemaPathProbe, event); err != nil {
				metrics.IncDropped("schema")
				log.Printf("probe schema validation failed: %v", err)
				continue
			}
			if err := writers.EmitProbe(event); err != nil {
				metrics.IncDropped("emit")
				log.Printf("probe emit failed: %v", err)
			}
		}

		if webhookExporter != nil {
			faultSample := attribution.FaultSample{
				IncidentID:    fmt.Sprintf("agent-%s-%d", sample.TraceID, idx),
				Timestamp:     now,
				Cluster:       *cluster,
				Namespace:     *namespace,
				Service:       *service,
				FaultLabel:    sample.FaultLabel,
				Confidence:    0.9,
				BurnRate:      2.0,
				WindowMinutes: 5,
				RequestID:     sample.RequestID,
				TraceID:       sample.TraceID,
			}
			attr := bayesAttributor.AttributeSample(faultSample)
			if err := webhookExporter.Send(attr); err != nil {
				log.Printf("webhook send failed: %v", err)
			}
		}

		if guard != nil {
			pct, exceeded, guardErr := guard.Evaluate()
			if guardErr != nil {
				log.Printf("overhead guard warning: %v", guardErr)
			} else {
				metrics.SetCPUOverhead(pct)
			}
			if exceeded {
				if disabledSignal, ok := generator.DisableHighestCost(); ok {
					log.Printf("overhead budget exceeded: disabled signal %s", disabledSignal)
					metrics.SetEnabledSignals(supportedSignals, generator.EnabledSignals())
				}
			}
		}

		metrics.SetHeartbeat(now)
		return nil
	}

	if *count > 0 {
		for idx := 0; idx < *count; idx++ {
			if err := emitOne(idx, time.Now().UTC()); err != nil {
				fmt.Fprintf(os.Stderr, "emit sample failed: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	ticker := time.NewTicker(time.Duration(*intervalMS) * time.Millisecond)
	defer ticker.Stop()

	idx := 0
	for {
		now := time.Now().UTC()
		if err := emitOne(idx, now); err != nil {
			fmt.Fprintf(os.Stderr, "emit sample failed: %v\n", err)
			os.Exit(1)
		}
		idx++

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func parseCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v == "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

func chooseEnabledSignals(configSignals []string, disabled []string, supported []string) []string {
	disabledSet := make(map[string]struct{}, len(disabled))
	for _, signal := range disabled {
		disabledSet[signal] = struct{}{}
	}

	supportedSet := make(map[string]struct{}, len(supported))
	for _, signal := range supported {
		supportedSet[signal] = struct{}{}
	}

	selected := make([]string, 0)
	if len(configSignals) == 0 {
		for _, signal := range supported {
			if _, blocked := disabledSet[signal]; blocked {
				continue
			}
			selected = append(selected, signal)
		}
		return selected
	}

	for _, signal := range configSignals {
		if _, ok := supportedSet[signal]; !ok {
			continue
		}
		if _, blocked := disabledSet[signal]; blocked {
			continue
		}
		selected = append(selected, signal)
	}

	if len(selected) == 0 {
		for _, signal := range supported {
			if _, blocked := disabledSet[signal]; blocked {
				continue
			}
			selected = append(selected, signal)
		}
	}

	return selected
}
