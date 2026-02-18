package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/collector"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/otel"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func main() {
	var (
		cluster   = flag.String("cluster", "local", "cluster name")
		namespace = flag.String("namespace", "default", "namespace")
		workload  = flag.String("workload", "llm-slo-agent", "workload")
		service   = flag.String("service", "agent", "service")
		node      = flag.String("k8s-node", "unknown-node", "node label")

		scenario   = flag.String("scenario", "baseline", "synthetic scenario name")
		count      = flag.Int("count", 0, "sample count (0 = stream mode)")
		intervalMS = flag.Int("interval-ms", 1000, "emit interval for stream mode")

		outputMode   = flag.String("output", "stdout", "output mode: stdout|jsonl|otlp")
		outputPath   = flag.String("output-path", "artifacts/agent/slo-events.jsonl", "output file when output=jsonl")
		otlpEndpoint = flag.String(
			"otlp-endpoint",
			"http://otel-collector.observability.svc.cluster.local:4318/v1/logs",
			"OTLP/HTTP logs endpoint when output=otlp",
		)
		otlpTimeoutMS = flag.Int("otlp-timeout-ms", 5000, "OTLP export timeout in milliseconds")

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

	if *intervalMS <= 0 {
		fmt.Fprintln(os.Stderr, "interval-ms must be > 0")
		os.Exit(1)
	}

	sink, closeWriter, err := openOutput(
		*outputMode,
		*outputPath,
		*otlpEndpoint,
		time.Duration(*otlpTimeoutMS)*time.Millisecond,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open output failed: %v\n", err)
		os.Exit(1)
	}
	defer closeWriter()

	var heartbeatUnix atomic.Int64
	heartbeatUnix.Store(time.Now().Unix())
	startMetricsServer(*metricsBind, &heartbeatUnix)

	schemaPath := filepath.Join("docs", "contracts", "v1", "slo-event.schema.json")
	meta := collector.SampleMeta{
		Cluster:   *cluster,
		Namespace: *namespace,
		Workload:  *workload,
		Service:   *service,
		Node:      *node,
	}

	if *count > 0 {
		samples, err := collector.GenerateSyntheticSamples(*scenario, *count, time.Now().UTC(), meta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "generate synthetic samples failed: %v\n", err)
			os.Exit(1)
		}
		if err := emitSamples(sink, schemaPath, samples); err != nil {
			fmt.Fprintf(os.Stderr, "emit samples failed: %v\n", err)
			os.Exit(1)
		}
		heartbeatUnix.Store(time.Now().Unix())
		return
	}

	ticker := time.NewTicker(time.Duration(*intervalMS) * time.Millisecond)
	defer ticker.Stop()
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	idx := 0
	for {
		sample, err := collector.BuildSyntheticSample(*scenario, idx, time.Now().UTC(), meta)
		if err != nil {
			fmt.Fprintf(os.Stderr, "build sample failed: %v\n", err)
			os.Exit(1)
		}
		if err := emitSamples(sink, schemaPath, []collector.RawSample{sample}); err != nil {
			fmt.Fprintf(os.Stderr, "emit sample failed: %v\n", err)
			os.Exit(1)
		}
		heartbeatUnix.Store(time.Now().Unix())
		idx++

		select {
		case <-ticker.C:
		case <-sigCh:
			return
		}
	}
}

func startMetricsServer(bind string, heartbeat *atomic.Int64) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		ts := heartbeat.Load()
		_, _ = fmt.Fprintf(w, "# HELP llm_slo_agent_heartbeat Unix timestamp of latest emitted sample.\n")
		_, _ = fmt.Fprintf(w, "# TYPE llm_slo_agent_heartbeat gauge\n")
		_, _ = fmt.Fprintf(w, "llm_slo_agent_heartbeat %d\n", ts)
		_, _ = fmt.Fprintf(w, "# HELP llm_slo_agent_up Agent process liveness.\n")
		_, _ = fmt.Fprintf(w, "# TYPE llm_slo_agent_up gauge\n")
		_, _ = fmt.Fprintf(w, "llm_slo_agent_up 1\n")
	})

	server := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "metrics server failed: %v\n", err)
		}
	}()
}

func emitSamples(sink eventSink, schemaPath string, samples []collector.RawSample) error {
	for _, sample := range samples {
		events := collector.NormalizeSample(sample)
		for _, event := range events {
			if err := schema.ValidateAgainstSchema(schemaPath, event); err != nil {
				return err
			}
			if err := sink.Emit(event); err != nil {
				return err
			}
		}
	}
	return nil
}

type eventSink interface {
	Emit(schema.SLOEvent) error
}

type jsonEventSink struct {
	encoder *json.Encoder
}

func (s *jsonEventSink) Emit(event schema.SLOEvent) error {
	return s.encoder.Encode(event)
}

type otlpEventSink struct {
	exporter *otel.SLOEventExporter
}

func (s *otlpEventSink) Emit(event schema.SLOEvent) error {
	return s.exporter.ExportBatch([]schema.SLOEvent{event})
}

func openOutput(
	mode string,
	path string,
	otlpEndpoint string,
	otlpTimeout time.Duration,
) (eventSink, func(), error) {
	switch mode {
	case "stdout":
		return &jsonEventSink{encoder: json.NewEncoder(os.Stdout)}, func() {}, nil
	case "jsonl":
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, func() {}, err
		}
		file, err := os.Create(path)
		if err != nil {
			return nil, func() {}, err
		}
		return &jsonEventSink{encoder: json.NewEncoder(file)}, func() {
			_ = file.Close()
		}, nil
	case "otlp":
		exporter := otel.NewSLOEventExporter(
			otlpEndpoint,
			"llm-slo-ebpf-toolkit",
			"llm-slo-ebpf-toolkit/agent",
			otlpTimeout,
		)
		return &otlpEventSink{exporter: exporter}, func() {}, nil
	default:
		return nil, func() {}, fmt.Errorf("unsupported output mode %q", mode)
	}
}
