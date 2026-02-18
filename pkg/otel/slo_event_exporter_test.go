package otel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func TestSLOEventExporterExportBatch(t *testing.T) {
	var captured logsPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	exporter := NewSLOEventExporter(server.URL, "llm-slo", "collector", 2*time.Second)
	err := exporter.ExportBatch([]schema.SLOEvent{
		{
			EventID:   "ev-1",
			Timestamp: time.Now().UTC(),
			Cluster:   "local",
			Namespace: "default",
			Workload:  "gateway",
			Service:   "chat",
			RequestID: "req-1",
			TraceID:   "trace-1",
			SLIName:   "ttft_ms",
			SLIValue:  123.4,
			Unit:      "ms",
			Status:    "warning",
			Labels: map[string]string{
				"fault_label": "dns_latency",
			},
		},
	})
	if err != nil {
		t.Fatalf("export batch: %v", err)
	}

	if len(captured.ResourceLogs) != 1 {
		t.Fatalf("expected 1 resource log, got %d", len(captured.ResourceLogs))
	}
	records := captured.ResourceLogs[0].ScopeLogs[0].LogRecords
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}
	if records[0].SeverityText != "WARN" {
		t.Fatalf("expected WARN severity, got %s", records[0].SeverityText)
	}
}

func TestSLOEventExporterNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	exporter := NewSLOEventExporter(server.URL, "", "", 2*time.Second)
	err := exporter.ExportBatch([]schema.SLOEvent{{EventID: "ev-1", SLIName: "ttft_ms"}})
	if err == nil {
		t.Fatal("expected non-2xx error")
	}
}
