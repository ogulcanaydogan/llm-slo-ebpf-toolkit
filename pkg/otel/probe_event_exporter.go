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

// ProbeEventExporter sends normalized probe events to an OTLP/HTTP logs endpoint.
type ProbeEventExporter struct {
	endpoint    string
	serviceName string
	scopeName   string
	client      *http.Client
}

// NewProbeEventExporter constructs an OTLP/HTTP logs exporter for probe events.
func NewProbeEventExporter(
	endpoint string,
	serviceName string,
	scopeName string,
	timeout time.Duration,
) *ProbeEventExporter {
	if serviceName == "" {
		serviceName = "llm-slo-ebpf-toolkit"
	}
	if scopeName == "" {
		scopeName = "llm-slo-ebpf-toolkit/agent"
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &ProbeEventExporter{
		endpoint:    endpoint,
		serviceName: serviceName,
		scopeName:   scopeName,
		client:      &http.Client{Timeout: timeout},
	}
}

// ExportBatch posts one OTLP payload that contains all provided probe events.
func (e *ProbeEventExporter) ExportBatch(events []schema.ProbeEventV1) error {
	if len(events) == 0 {
		return nil
	}
	if e.endpoint == "" {
		return fmt.Errorf("otlp endpoint is required")
	}

	payload := buildProbeLogsPayload(e.serviceName, e.scopeName, events)
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

func buildProbeLogsPayload(serviceName string, scopeName string, events []schema.ProbeEventV1) logsPayload {
	records := make([]logRecord, 0, len(events))
	for _, event := range events {
		records = append(records, toProbeLogRecord(event))
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

func toProbeLogRecord(event schema.ProbeEventV1) logRecord {
	now := strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
	ts := strconv.FormatInt(event.TSUnixNano, 10)
	if event.TSUnixNano <= 0 {
		ts = now
	}

	attrs := []keyValue{
		strAttribute("signal", event.Signal),
		strAttribute("node", event.Node),
		strAttribute("namespace", event.Namespace),
		strAttribute("pod", event.Pod),
		strAttribute("container", event.Container),
		doubleAttribute("pid", float64(event.PID)),
		doubleAttribute("tid", float64(event.TID)),
		doubleAttribute("value", event.Value),
		strAttribute("unit", event.Unit),
		strAttribute("status", event.Status),
	}
	if event.TraceID != "" {
		attrs = append(attrs, strAttribute("trace.id", event.TraceID))
	}
	if event.SpanID != "" {
		attrs = append(attrs, strAttribute("span.id", event.SpanID))
	}
	if event.ConnTuple != nil {
		attrs = append(attrs,
			strAttribute("net.src.ip", event.ConnTuple.SrcIP),
			strAttribute("net.dst.ip", event.ConnTuple.DstIP),
			doubleAttribute("net.src.port", float64(event.ConnTuple.SrcPort)),
			doubleAttribute("net.dst.port", float64(event.ConnTuple.DstPort)),
			strAttribute("net.transport", event.ConnTuple.Protocol),
		)
	}
	if event.Errno != nil {
		attrs = append(attrs, doubleAttribute("errno", float64(*event.Errno)))
	}
	if event.Confidence != nil {
		attrs = append(attrs, doubleAttribute("correlation.confidence", *event.Confidence))
	}

	return logRecord{
		TimeUnixNano:         ts,
		ObservedTimeUnixNano: now,
		SeverityText:         severityFromStatus(event.Status),
		Body: anyValue{
			StringValue: fmt.Sprintf(
				"signal=%s value=%.6f status=%s pod=%s",
				event.Signal,
				event.Value,
				event.Status,
				event.Pod,
			),
		},
		Attributes: attrs,
	}
}
