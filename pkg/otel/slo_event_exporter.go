package otel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/schema"
)

// SLOEventExporter sends normalized SLO events to an OTLP/HTTP logs endpoint.
type SLOEventExporter struct {
	endpoint    string
	serviceName string
	scopeName   string
	client      *http.Client
}

// NewSLOEventExporter constructs an OTLP/HTTP logs exporter.
func NewSLOEventExporter(
	endpoint string,
	serviceName string,
	scopeName string,
	timeout time.Duration,
) *SLOEventExporter {
	if serviceName == "" {
		serviceName = "llm-slo-ebpf-toolkit"
	}
	if scopeName == "" {
		scopeName = "llm-slo-ebpf-toolkit/collector"
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &SLOEventExporter{
		endpoint:    endpoint,
		serviceName: serviceName,
		scopeName:   scopeName,
		client:      &http.Client{Timeout: timeout},
	}
}

// ExportBatch posts one OTLP payload that contains all provided SLO events.
func (e *SLOEventExporter) ExportBatch(events []schema.SLOEvent) error {
	if len(events) == 0 {
		return nil
	}
	if e.endpoint == "" {
		return fmt.Errorf("otlp endpoint is required")
	}

	payload := buildLogsPayload(e.serviceName, e.scopeName, events)
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal otlp payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, e.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build otlp request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("send otlp payload: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("otlp endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

type logsPayload struct {
	ResourceLogs []resourceLogs `json:"resourceLogs"`
}

type resourceLogs struct {
	Resource  resource    `json:"resource"`
	ScopeLogs []scopeLogs `json:"scopeLogs"`
}

type resource struct {
	Attributes []keyValue `json:"attributes"`
}

type scopeLogs struct {
	Scope      scope       `json:"scope"`
	LogRecords []logRecord `json:"logRecords"`
}

type scope struct {
	Name string `json:"name"`
}

type logRecord struct {
	TimeUnixNano         string     `json:"timeUnixNano"`
	ObservedTimeUnixNano string     `json:"observedTimeUnixNano"`
	SeverityText         string     `json:"severityText"`
	Body                 anyValue   `json:"body"`
	Attributes           []keyValue `json:"attributes"`
}

type keyValue struct {
	Key   string   `json:"key"`
	Value anyValue `json:"value"`
}

type anyValue struct {
	StringValue string   `json:"stringValue,omitempty"`
	DoubleValue *float64 `json:"doubleValue,omitempty"`
}

func buildLogsPayload(serviceName string, scopeName string, events []schema.SLOEvent) logsPayload {
	records := make([]logRecord, 0, len(events))
	for _, event := range events {
		records = append(records, toLogRecord(event))
	}

	return logsPayload{
		ResourceLogs: []resourceLogs{
			{
				Resource: resource{
					Attributes: []keyValue{
						strAttribute("service.name", serviceName),
					},
				},
				ScopeLogs: []scopeLogs{
					{
						Scope:      scope{Name: scopeName},
						LogRecords: records,
					},
				},
			},
		},
	}
}

func toLogRecord(event schema.SLOEvent) logRecord {
	now := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	ts := strconv.FormatInt(event.Timestamp.UnixNano(), 10)
	if event.Timestamp.IsZero() {
		ts = now
	}

	attrs := []keyValue{
		strAttribute("event.id", event.EventID),
		strAttribute("cluster", event.Cluster),
		strAttribute("namespace", event.Namespace),
		strAttribute("workload", event.Workload),
		strAttribute("service", event.Service),
		strAttribute("request.id", event.RequestID),
		strAttribute("trace.id", event.TraceID),
		strAttribute("sli.name", event.SLIName),
		doubleAttribute("sli.value", event.SLIValue),
		strAttribute("sli.unit", event.Unit),
		strAttribute("sli.status", event.Status),
	}
	for key, value := range event.Labels {
		attrs = append(attrs, strAttribute("label."+key, value))
	}

	return logRecord{
		TimeUnixNano:         ts,
		ObservedTimeUnixNano: now,
		SeverityText:         severityFromStatus(event.Status),
		Body: anyValue{
			StringValue: fmt.Sprintf(
				"sli=%s value=%.6f status=%s service=%s",
				event.SLIName,
				event.SLIValue,
				event.Status,
				event.Service,
			),
		},
		Attributes: attrs,
	}
}

func strAttribute(key string, value string) keyValue {
	return keyValue{Key: key, Value: anyValue{StringValue: value}}
}

func doubleAttribute(key string, value float64) keyValue {
	v := value
	return keyValue{Key: key, Value: anyValue{DoubleValue: &v}}
}

func severityFromStatus(status string) string {
	switch status {
	case "breach", "error":
		return "ERROR"
	case "warning":
		return "WARN"
	default:
		return "INFO"
	}
}
