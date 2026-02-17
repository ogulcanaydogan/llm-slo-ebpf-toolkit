package attribution

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadSamplesFromJSONL(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "samples.jsonl")

	samples := []FaultSample{
		{
			IncidentID:    "inc-1",
			Timestamp:     time.Now().UTC(),
			Cluster:       "local",
			Namespace:     "default",
			Service:       "chat",
			FaultLabel:    "provider_throttle",
			Confidence:    0.9,
			BurnRate:      2.0,
			WindowMinutes: 5,
			RequestID:     "req-1",
			TraceID:       "trace-1",
		},
		{
			IncidentID:    "inc-2",
			Timestamp:     time.Now().UTC(),
			Cluster:       "local",
			Namespace:     "default",
			Service:       "chat",
			FaultLabel:    "dns_latency",
			Confidence:    0.8,
			BurnRate:      1.8,
			WindowMinutes: 5,
			RequestID:     "req-2",
			TraceID:       "trace-2",
		},
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	enc := json.NewEncoder(file)
	for _, sample := range samples {
		if err := enc.Encode(sample); err != nil {
			t.Fatalf("encode fixture: %v", err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close fixture: %v", err)
	}

	loaded, err := LoadSamplesFromJSONL(path)
	if err != nil {
		t.Fatalf("load samples: %v", err)
	}
	if len(loaded) != len(samples) {
		t.Fatalf("expected %d samples, got %d", len(samples), len(loaded))
	}
}

func TestLoadSamplesFromJSONLEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "samples.jsonl")
	if err := os.WriteFile(path, []byte("\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	if _, err := LoadSamplesFromJSONL(path); err == nil {
		t.Fatal("expected error for empty JSONL input")
	}
}
