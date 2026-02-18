package signals

import (
	"os"
	"runtime"
)

// DetectCapabilityMode chooses full CO-RE mode when BTF is available.
func DetectCapabilityMode() CapabilityMode {
	if runtime.GOOS != "linux" {
		return CapabilityBCCDegraded
	}
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err == nil {
		return CapabilityCoreFull
	}
	return CapabilityBCCDegraded
}

// ParseCapabilityMode parses mode flag values.
func ParseCapabilityMode(v string) CapabilityMode {
	switch v {
	case string(CapabilityCoreFull):
		return CapabilityCoreFull
	case string(CapabilityBCCDegraded):
		return CapabilityBCCDegraded
	default:
		return DetectCapabilityMode()
	}
}
