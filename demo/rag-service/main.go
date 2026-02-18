package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/correlation"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/otel/processor/ebpfcorrelator"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/semconv"
	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/slo"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	stdouttrace "go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	otelsemconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type corpusDoc struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

type chatRequest struct {
	RequestID string `json:"request_id"`
	Prompt    string `json:"prompt"`
	Profile   string `json:"profile"`
	Seed      int64  `json:"seed"`
	MaxTokens int    `json:"max_tokens"`
	Stream    bool   `json:"stream"`
}

type retrievalPlan struct {
	DNSMS       float64
	NetworkMS   float64
	VectorDBMS  float64
	WarmupMS    int
	CadenceMS   int
	SelectCount int
}

type retrievalResult struct {
	SelectedTitles []string
	Breakdown      slo.RetrievalBreakdown
	Plan           retrievalPlan
}

type streamEvent struct {
	Type            string   `json:"type"`
	RequestID       string   `json:"request_id,omitempty"`
	TraceID         string   `json:"trace_id,omitempty"`
	Profile         string   `json:"profile,omitempty"`
	Token           string   `json:"token,omitempty"`
	Index           int      `json:"index,omitempty"`
	SelectedTitles  []string `json:"selected_titles,omitempty"`
	TTFTMS          float64  `json:"ttft_ms,omitempty"`
	TokensPerSecond float64  `json:"tokens_per_sec,omitempty"`
	VectorDBMS      float64  `json:"retrieval_vectordb_ms,omitempty"`
	NetworkMS       float64  `json:"retrieval_network_ms,omitempty"`
	DNSMS           float64  `json:"retrieval_dns_ms,omitempty"`
}

type appMetrics struct {
	ttftHist          prometheus.Histogram
	tokensPerSecHist  prometheus.Histogram
	retrievalVectorDB prometheus.Histogram
	retrievalNetwork  prometheus.Histogram
	retrievalDNS      prometheus.Histogram
	requestTotal      *prometheus.CounterVec
	correlationTotal  *prometheus.CounterVec
}

type appServer struct {
	tracer     trace.Tracer
	correlator ebpfcorrelator.Correlator
	docs       []corpusDoc
	metrics    appMetrics
}

func main() {
	bind := flag.String("bind", ":8080", "HTTP bind address")
	metricsBind := flag.String("metrics-bind", ":2113", "metrics bind address")
	serviceName := flag.String("service-name", "rag-service", "service.name for telemetry")
	otlpEndpoint := flag.String("otlp-endpoint", envOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", ""), "OTLP endpoint host:port")
	fixtures := flag.String("fixtures", filepath.Join("demo", "rag-service", "fixtures", "corpus.json"), "fixtures corpus path")
	flag.Parse()

	docs, err := loadCorpus(*fixtures)
	if err != nil {
		log.Fatalf("load fixtures: %v", err)
	}

	shutdown, err := setupTracerProvider(*serviceName, *otlpEndpoint)
	if err != nil {
		log.Fatalf("setup tracer provider: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdown(ctx)
	}()

	registry := prometheus.NewRegistry()
	metrics := newMetrics(registry)
	server := appServer{
		tracer:     otel.Tracer("llm-slo/rag-service"),
		correlator: ebpfcorrelator.New(),
		docs:       docs,
		metrics:    metrics,
	}

	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	metricsMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	go func() {
		if err := http.ListenAndServe(*metricsBind, metricsMux); err != nil {
			log.Printf("metrics server stopped: %v", err)
		}
	}()

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	apiMux.HandleFunc("/chat", server.handleChat)

	log.Printf("rag service listening on %s (metrics %s)", *bind, *metricsBind)
	if err := http.ListenAndServe(*bind, apiMux); err != nil {
		log.Fatal(err)
	}
}

func newMetrics(reg prometheus.Registerer) appMetrics {
	m := appMetrics{
		ttftHist: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_slo_ttft_ms",
			Help:    "Time-to-first-token in milliseconds.",
			Buckets: []float64{50, 100, 200, 400, 800, 1200, 2000},
		}),
		tokensPerSecHist: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_slo_tokens_per_sec",
			Help:    "Generated token throughput.",
			Buckets: []float64{5, 10, 20, 30, 40, 60, 80},
		}),
		retrievalVectorDB: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_slo_retrieval_vectordb_ms",
			Help:    "Vector DB retrieval latency component.",
			Buckets: []float64{5, 10, 20, 40, 80, 160, 320, 640},
		}),
		retrievalNetwork: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_slo_retrieval_network_ms",
			Help:    "Retrieval network latency component.",
			Buckets: []float64{2, 5, 10, 20, 40, 80, 160},
		}),
		retrievalDNS: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "llm_slo_retrieval_dns_ms",
			Help:    "Retrieval DNS latency component.",
			Buckets: []float64{1, 2, 5, 10, 20, 40, 80, 160},
		}),
		requestTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_slo_requests_total",
			Help: "Total chat requests by status/profile.",
		}, []string{"status", "profile"}),
		correlationTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "llm_slo_correlation_total",
			Help: "Correlation decisions by tier and enrichment flag.",
		}, []string{"tier", "enriched"}),
	}

	reg.MustRegister(
		m.ttftHist,
		m.tokensPerSecHist,
		m.retrievalVectorDB,
		m.retrievalNetwork,
		m.retrievalDNS,
		m.requestTotal,
		m.correlationTotal,
	)
	return m
}

func (a appServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		a.metrics.requestTotal.WithLabelValues("bad_request", "unknown").Inc()
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		http.Error(w, "prompt is required", http.StatusBadRequest)
		a.metrics.requestTotal.WithLabelValues("bad_request", "unknown").Inc()
		return
	}

	if req.Profile == "" {
		req.Profile = "rag_medium"
	}
	if req.Seed == 0 {
		req.Seed = 42
	}
	if req.MaxTokens <= 0 {
		req.MaxTokens = 64
	}
	if req.RequestID == "" {
		req.RequestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}

	ctx, span := a.tracer.Start(ctx, "chat.request",
		trace.WithAttributes(
			attribute.String("request.id", req.RequestID),
			attribute.String("llm.profile", req.Profile),
			attribute.Int64("llm.seed", req.Seed),
		),
	)
	defer span.End()

	requestStart := time.Now().UTC()
	traceID := span.SpanContext().TraceID().String()

	retrievalCtx, retrievalSpan := a.tracer.Start(ctx, "chat.retrieval")
	retrieval, err := a.simulateRetrieval(retrievalCtx, req)
	retrievalSpan.SetAttributes(
		attribute.Float64(semconv.AttrRetrievalVectorDB, retrieval.Breakdown.VectorDBMS),
		attribute.Float64(semconv.AttrRetrievalNetworkMS, retrieval.Breakdown.NetworkMS),
		attribute.Float64(semconv.AttrRetrievalDNSMS, retrieval.Breakdown.DNSMS),
		attribute.Int("retrieval.selected_docs", len(retrieval.SelectedTitles)),
	)
	retrievalSpan.End()
	if err != nil {
		span.RecordError(err)
		http.Error(w, "retrieval failed", http.StatusInternalServerError)
		a.metrics.requestTotal.WithLabelValues("error", req.Profile).Inc()
		return
	}

	attrs, decision := a.correlator.EnrichDNSAttributes(
		nil,
		correlation.SpanRef{
			TraceID:   traceID,
			Service:   "rag-service",
			Node:      "demo-node",
			Pod:       "demo-rag-service",
			PID:       os.Getpid(),
			Timestamp: requestStart,
		},
		correlation.SignalRef{
			Signal:    "dns_latency_ms",
			TraceID:   traceID,
			Service:   "rag-service",
			Node:      "demo-node",
			Pod:       "demo-rag-service",
			PID:       os.Getpid(),
			Timestamp: time.Now().UTC(),
			Value:     retrieval.Breakdown.DNSMS,
		},
	)
	if decision.Matched {
		enriched := "false"
		if decision.Confidence >= a.correlator.EnrichmentThreshold {
			enriched = "true"
		}
		a.metrics.correlationTotal.WithLabelValues(decision.Tier, enriched).Inc()
	}
	for key, val := range attrs {
		span.SetAttributes(attribute.Float64(key, val))
	}
	if decision.Tier != "" {
		span.SetAttributes(attribute.String("llm.ebpf.correlation_tier", decision.Tier))
	}

	genCtx, genSpan := a.tracer.Start(ctx, "chat.generation")
	tokens := generateTokens(req.Prompt, req.MaxTokens, req.Seed)
	genSpan.SetAttributes(attribute.Int("llm.tokens.count", len(tokens)))

	warmup := time.Duration(retrieval.Plan.WarmupMS) * time.Millisecond
	cadence := time.Duration(retrieval.Plan.CadenceMS) * time.Millisecond

	if req.Stream {
		if err := streamResponse(w, req, traceID, retrieval, tokens, requestStart, warmup, cadence, a.metrics, span); err != nil {
			genSpan.RecordError(err)
			span.RecordError(err)
			http.Error(w, "stream write failed", http.StatusInternalServerError)
			a.metrics.requestTotal.WithLabelValues("error", req.Profile).Inc()
			genSpan.End()
			return
		}
	} else {
		if err := nonStreamResponse(w, req, traceID, retrieval, tokens, requestStart, warmup, cadence, a.metrics, span); err != nil {
			genSpan.RecordError(err)
			span.RecordError(err)
			http.Error(w, "response failed", http.StatusInternalServerError)
			a.metrics.requestTotal.WithLabelValues("error", req.Profile).Inc()
			genSpan.End()
			return
		}
	}

	genSpan.End()
	a.metrics.requestTotal.WithLabelValues("ok", req.Profile).Inc()
	_ = genCtx
}

func streamResponse(
	w http.ResponseWriter,
	req chatRequest,
	traceID string,
	retrieval retrievalResult,
	tokens []string,
	requestStart time.Time,
	warmup time.Duration,
	cadence time.Duration,
	metrics appMetrics,
	span trace.Span,
) error {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming requires http.Flusher")
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	enc := json.NewEncoder(w)

	meta := streamEvent{
		Type:           "meta",
		RequestID:      req.RequestID,
		TraceID:        traceID,
		Profile:        req.Profile,
		SelectedTitles: retrieval.SelectedTitles,
		VectorDBMS:     retrieval.Breakdown.VectorDBMS,
		NetworkMS:      retrieval.Breakdown.NetworkMS,
		DNSMS:          retrieval.Breakdown.DNSMS,
	}
	if err := enc.Encode(meta); err != nil {
		return err
	}
	flusher.Flush()

	firstTokenAt := time.Time{}
	lastTokenAt := time.Time{}

	for idx, token := range tokens {
		if idx == 0 {
			if err := wait(warmup); err != nil {
				return err
			}
			firstTokenAt = time.Now().UTC()
		} else {
			if err := wait(cadence); err != nil {
				return err
			}
		}
		lastTokenAt = time.Now().UTC()

		event := streamEvent{
			Type:      "token",
			RequestID: req.RequestID,
			TraceID:   traceID,
			Token:     token,
			Index:     idx,
		}
		if err := enc.Encode(event); err != nil {
			return err
		}
		flusher.Flush()
	}

	snapshot, err := slo.Calculate(slo.Timing{
		RequestStart: requestStart,
		FirstTokenAt: firstTokenAt,
		LastTokenAt:  lastTokenAt,
		TokenCount:   len(tokens),
	}, retrieval.Breakdown)
	if err != nil {
		return err
	}
	recordSnapshot(metrics, snapshot)
	span.SetAttributes(
		attribute.Float64(semconv.AttrSLOTTFTMS, snapshot.TTFTMs),
		attribute.Float64(semconv.AttrSLOTokensPerSec, snapshot.TokensPerS),
		attribute.Float64(semconv.AttrRetrievalVectorDB, retrieval.Breakdown.VectorDBMS),
		attribute.Float64(semconv.AttrRetrievalNetworkMS, retrieval.Breakdown.NetworkMS),
		attribute.Float64(semconv.AttrRetrievalDNSMS, retrieval.Breakdown.DNSMS),
	)

	done := streamEvent{
		Type:            "done",
		RequestID:       req.RequestID,
		TraceID:         traceID,
		TTFTMS:          snapshot.TTFTMs,
		TokensPerSecond: snapshot.TokensPerS,
		VectorDBMS:      retrieval.Breakdown.VectorDBMS,
		NetworkMS:       retrieval.Breakdown.NetworkMS,
		DNSMS:           retrieval.Breakdown.DNSMS,
	}
	if err := enc.Encode(done); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func nonStreamResponse(
	w http.ResponseWriter,
	req chatRequest,
	traceID string,
	retrieval retrievalResult,
	tokens []string,
	requestStart time.Time,
	warmup time.Duration,
	cadence time.Duration,
	metrics appMetrics,
	span trace.Span,
) error {
	firstTokenAt := time.Now().UTC().Add(warmup)
	lastTokenAt := firstTokenAt.Add(time.Duration(len(tokens)-1) * cadence)

	snapshot, err := slo.Calculate(slo.Timing{
		RequestStart: requestStart,
		FirstTokenAt: firstTokenAt,
		LastTokenAt:  lastTokenAt,
		TokenCount:   len(tokens),
	}, retrieval.Breakdown)
	if err != nil {
		return err
	}
	recordSnapshot(metrics, snapshot)

	span.SetAttributes(
		attribute.Float64(semconv.AttrSLOTTFTMS, snapshot.TTFTMs),
		attribute.Float64(semconv.AttrSLOTokensPerSec, snapshot.TokensPerS),
		attribute.Float64(semconv.AttrRetrievalVectorDB, retrieval.Breakdown.VectorDBMS),
		attribute.Float64(semconv.AttrRetrievalNetworkMS, retrieval.Breakdown.NetworkMS),
		attribute.Float64(semconv.AttrRetrievalDNSMS, retrieval.Breakdown.DNSMS),
	)

	resp := map[string]interface{}{
		"request_id":            req.RequestID,
		"trace_id":              traceID,
		"profile":               req.Profile,
		"selected_titles":       retrieval.SelectedTitles,
		"response":              strings.Join(tokens, " "),
		"ttft_ms":               snapshot.TTFTMs,
		"tokens_per_sec":        snapshot.TokensPerS,
		"retrieval_vectordb_ms": retrieval.Breakdown.VectorDBMS,
		"retrieval_network_ms":  retrieval.Breakdown.NetworkMS,
		"retrieval_dns_ms":      retrieval.Breakdown.DNSMS,
	}
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp)
}

func recordSnapshot(metrics appMetrics, snapshot slo.Snapshot) {
	metrics.ttftHist.Observe(snapshot.TTFTMs)
	metrics.tokensPerSecHist.Observe(snapshot.TokensPerS)
	metrics.retrievalVectorDB.Observe(snapshot.Retrieval.VectorDBMS)
	metrics.retrievalNetwork.Observe(snapshot.Retrieval.NetworkMS)
	metrics.retrievalDNS.Observe(snapshot.Retrieval.DNSMS)
}

func (a appServer) simulateRetrieval(ctx context.Context, req chatRequest) (retrievalResult, error) {
	rng := rand.New(rand.NewSource(req.Seed + int64(hashPrompt(req.Prompt))))
	plan := planForRequest(req.Profile, req.Prompt, rng)
	docs := selectDocs(a.docs, plan.SelectCount, rng)

	if err := sleepWithContext(ctx, durationMS(plan.DNSMS)); err != nil {
		return retrievalResult{}, err
	}
	if err := sleepWithContext(ctx, durationMS(plan.NetworkMS)); err != nil {
		return retrievalResult{}, err
	}
	if err := sleepWithContext(ctx, durationMS(plan.VectorDBMS)); err != nil {
		return retrievalResult{}, err
	}

	titles := make([]string, 0, len(docs))
	for _, d := range docs {
		titles = append(titles, d.Title)
	}
	sort.Strings(titles)

	return retrievalResult{
		SelectedTitles: titles,
		Breakdown: slo.RetrievalBreakdown{
			VectorDBMS: plan.VectorDBMS,
			NetworkMS:  plan.NetworkMS,
			DNSMS:      plan.DNSMS,
		},
		Plan: plan,
	}, nil
}

func planForRequest(profile string, prompt string, rng *rand.Rand) retrievalPlan {
	if rng == nil {
		rng = rand.New(rand.NewSource(42))
	}

	switch profile {
	case "chat_short":
		return retrievalPlan{
			DNSMS:       2 + float64(rng.Intn(4)),
			NetworkMS:   4 + float64(rng.Intn(8)),
			VectorDBMS:  10 + float64(rng.Intn(20)),
			WarmupMS:    25 + rng.Intn(15),
			CadenceMS:   25 + rng.Intn(10),
			SelectCount: 2 + rng.Intn(2),
		}
	case "context_long":
		return retrievalPlan{
			DNSMS:       8 + float64(rng.Intn(8)),
			NetworkMS:   15 + float64(rng.Intn(20)),
			VectorDBMS:  70 + float64(rng.Intn(80)),
			WarmupMS:    50 + rng.Intn(30),
			CadenceMS:   45 + rng.Intn(20),
			SelectCount: 4 + rng.Intn(2),
		}
	default:
		return retrievalPlan{
			DNSMS:       5 + float64(rng.Intn(8)),
			NetworkMS:   10 + float64(rng.Intn(14)),
			VectorDBMS:  35 + float64(rng.Intn(45)),
			WarmupMS:    30 + rng.Intn(20),
			CadenceMS:   30 + rng.Intn(15),
			SelectCount: 3 + rng.Intn(2),
		}
	}
}

func generateTokens(prompt string, maxTokens int, seed int64) []string {
	if maxTokens < 1 {
		maxTokens = 1
	}
	if maxTokens > 256 {
		maxTokens = 256
	}

	rng := rand.New(rand.NewSource(seed + int64(hashPrompt(prompt))))
	base := sanitizeTokens(strings.Fields(prompt))
	vocab := []string{
		"reliability", "signal", "trace", "kernel", "latency", "attribution", "dns", "retrieval",
		"throughput", "incident", "confidence", "evidence", "burn", "slo", "token", "scheduler",
	}

	out := make([]string, 0, maxTokens)
	for _, token := range base {
		if len(out) == maxTokens {
			return out
		}
		out = append(out, token)
	}
	for len(out) < maxTokens {
		out = append(out, vocab[rng.Intn(len(vocab))])
	}
	return out
}

func sanitizeTokens(tokens []string) []string {
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		t := strings.Trim(strings.ToLower(token), " ,.!?;:\"()[]{}")
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func selectDocs(corpus []corpusDoc, n int, rng *rand.Rand) []corpusDoc {
	if n <= 0 || len(corpus) == 0 {
		return nil
	}
	if n > len(corpus) {
		n = len(corpus)
	}

	idx := rng.Perm(len(corpus))
	selected := make([]corpusDoc, 0, n)
	for i := 0; i < n; i++ {
		selected = append(selected, corpus[idx[i]])
	}
	return selected
}

func loadCorpus(path string) ([]corpusDoc, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var docs []corpusDoc
	if err := json.NewDecoder(f).Decode(&docs); err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("empty corpus")
	}
	return docs, nil
}

func setupTracerProvider(serviceName string, endpoint string) (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			otelsemconv.SchemaURL,
			otelsemconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	var exporter sdktrace.SpanExporter
	if strings.TrimSpace(endpoint) == "" {
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	} else {
		clean := strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://")
		exporter, err = otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(clean),
			otlptracegrpc.WithInsecure(),
		)
		if err != nil {
			return nil, err
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

func hashPrompt(prompt string) uint32 {
	h := fnv.New32a()
	_, _ = io.WriteString(h, prompt)
	return h.Sum32()
}

func durationMS(ms float64) time.Duration {
	return time.Duration(ms * float64(time.Millisecond))
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func wait(d time.Duration) error {
	if d <= 0 {
		return nil
	}
	if d > 10*time.Second {
		return errors.New("invalid wait duration")
	}
	time.Sleep(d)
	return nil
}

func envOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}

func parseIntOrDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
