package toolkitcfg

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ToolkitConfig mirrors config/toolkit.yaml.
type ToolkitConfig struct {
	APIVersion  string            `yaml:"apiVersion"`
	Kind        string            `yaml:"kind"`
	SignalSet   []string          `yaml:"signal_set"`
	Sampling    SamplingConfig    `yaml:"sampling"`
	Correlation CorrelationConfig `yaml:"correlation"`
	OTLP        OTLPConfig        `yaml:"otlp"`
	Safety      SafetyConfig      `yaml:"safety"`
}

// SamplingConfig controls event-rate limiting.
type SamplingConfig struct {
	EventsPerSecondLimit int `yaml:"events_per_second_limit"`
	BurstLimit           int `yaml:"burst_limit"`
}

// CorrelationConfig contains join-window tuning.
type CorrelationConfig struct {
	WindowMS int `yaml:"window_ms"`
}

// OTLPConfig contains collector endpoint settings.
type OTLPConfig struct {
	Endpoint string `yaml:"endpoint"`
}

// SafetyConfig configures runtime overhead limits.
type SafetyConfig struct {
	MaxOverheadPct float64 `yaml:"max_overhead_pct"`
}

// Default returns v1alpha1 defaults.
func Default() ToolkitConfig {
	return ToolkitConfig{
		APIVersion: "toolkit.llm-slo.dev/v1alpha1",
		Kind:       "ToolkitConfig",
		SignalSet: []string{
			"dns_latency_ms",
			"tcp_retransmits_total",
			"runqueue_delay_ms",
			"connect_latency_ms",
			"tls_handshake_ms",
			"cpu_steal_pct",
		},
		Sampling: SamplingConfig{
			EventsPerSecondLimit: 10000,
			BurstLimit:           20000,
		},
		Correlation: CorrelationConfig{
			WindowMS: 2000,
		},
		OTLP: OTLPConfig{
			Endpoint: "http://otel-collector:4317",
		},
		Safety: SafetyConfig{
			MaxOverheadPct: 5,
		},
	}
}

// Load parses and normalizes a toolkit config file.
func Load(path string) (ToolkitConfig, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("unmarshal config %s: %w", path, err)
	}
	normalize(&cfg)
	return cfg, nil
}

func normalize(cfg *ToolkitConfig) {
	if len(cfg.SignalSet) == 0 {
		cfg.SignalSet = Default().SignalSet
	}
	if cfg.Sampling.EventsPerSecondLimit <= 0 {
		cfg.Sampling.EventsPerSecondLimit = Default().Sampling.EventsPerSecondLimit
	}
	if cfg.Sampling.BurstLimit <= 0 {
		cfg.Sampling.BurstLimit = Default().Sampling.BurstLimit
	}
	if cfg.Correlation.WindowMS <= 0 {
		cfg.Correlation.WindowMS = Default().Correlation.WindowMS
	}
	if cfg.OTLP.Endpoint == "" {
		cfg.OTLP.Endpoint = Default().OTLP.Endpoint
	}
	if cfg.Safety.MaxOverheadPct <= 0 {
		cfg.Safety.MaxOverheadPct = Default().Safety.MaxOverheadPct
	}
	if cfg.APIVersion == "" {
		cfg.APIVersion = Default().APIVersion
	}
	if cfg.Kind == "" {
		cfg.Kind = Default().Kind
	}
}
