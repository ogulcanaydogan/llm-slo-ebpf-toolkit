package otel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

func TestProbeEventExporterExportBatch(t *testing.T) {
	var captured logsPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	exporter := NewProbeEventExporter(server.URL, "llm-slo", "agent", 2*time.Second)
	errno := 110
	err := exporter.ExportBatch([]schema.ProbeEventV1{
		{
			TSUnixNano: time.Now().UTC().UnixNano(),
			Signal:     "connect_errors_total",
			Node:       "kind-worker",
			Namespace:  "default",
			Pod:        "rag-0",
			Container:  "rag",
			PID:        1234,
			TID:        1234,
			Value:      2,
			Unit:       "count",
			Status:     "warning",
			Errno:      &errno,
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
