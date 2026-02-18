package signals

import (
	"fmt"
	"os"
	"strings"
)

// Metadata is the canonical workload identity attached to probe events.
type Metadata struct {
	Node      string
	Namespace string
	Pod       string
	Container string
	Service   string
	Workload  string
	PID       int
	TID       int
	TraceID   string
	SpanID    string
}

// MetadataEnricher enriches probe metadata before emission.
type MetadataEnricher interface {
	Enrich(meta Metadata) Metadata
}

// StaticMetadataEnricher fills known defaults.
type StaticMetadataEnricher struct {
	Defaults Metadata
}

// Enrich applies defaults for missing values.
func (e StaticMetadataEnricher) Enrich(meta Metadata) Metadata {
	out := meta
	if out.Node == "" {
		out.Node = fallback(e.Defaults.Node, "unknown-node")
	}
	if out.Namespace == "" {
		out.Namespace = fallback(e.Defaults.Namespace, "default")
	}
	if out.Pod == "" {
		out.Pod = fallback(e.Defaults.Pod, "unknown-pod")
	}
	if out.Container == "" {
		out.Container = fallback(e.Defaults.Container, "unknown-container")
	}
	if out.Service == "" {
		out.Service = e.Defaults.Service
	}
	if out.Workload == "" {
		out.Workload = e.Defaults.Workload
	}
	if out.PID <= 0 {
		out.PID = maxInt(e.Defaults.PID, os.Getpid())
	}
	if out.TID <= 0 {
		if e.Defaults.TID > 0 {
			out.TID = e.Defaults.TID
		} else {
			out.TID = out.PID
		}
	}
	if out.TraceID == "" {
		out.TraceID = e.Defaults.TraceID
	}
	if out.SpanID == "" {
		out.SpanID = e.Defaults.SpanID
	}
	return out
}

// ProcMetadataEnricher attempts lightweight cgroup-based identity recovery.
type ProcMetadataEnricher struct {
	Next MetadataEnricher
}

// Enrich derives pod/container labels from /proc/<pid>/cgroup when possible.
func (e ProcMetadataEnricher) Enrich(meta Metadata) Metadata {
	out := meta
	if out.PID > 0 {
		if pod, container := deriveFromCgroup(out.PID); pod != "" && out.Pod == "" {
			out.Pod = pod
			if out.Container == "" {
				out.Container = container
			}
		}
	}
	if e.Next != nil {
		return e.Next.Enrich(out)
	}
	return out
}

func deriveFromCgroup(pid int) (string, string) {
	path := fmt.Sprintf("/proc/%d/cgroup", pid)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(string(data), "/")
	var pod string
	var container string
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if pod == "" && strings.HasPrefix(p, "pod") {
			pod = normalizePodLabel(strings.TrimPrefix(p, "pod"))
			continue
		}
		if container == "" && likelyContainerID(p) {
			container = shortID(p, 12)
		}
	}
	return pod, container
}

func normalizePodLabel(raw string) string {
	if raw == "" {
		return raw
	}
	label := strings.Trim(raw, ".slice")
	label = strings.ReplaceAll(label, "_", "-")
	return label
}

func likelyContainerID(v string) bool {
	if len(v) < 12 {
		return false
	}
	hexLen := 0
	for _, ch := range v {
		if (ch >= 'a' && ch <= 'f') || (ch >= '0' && ch <= '9') {
			hexLen++
		}
	}
	return hexLen >= 12
}

func shortID(v string, max int) string {
	if len(v) <= max {
		return v
	}
	return v[:max]
}

func fallback(value string, def string) string {
	if value == "" {
		return def
	}
	return value
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
