package collector

import (
	"fmt"
	"os"
	"runtime"

	"github.com/cilium/ebpf"
)

// DependencyMarker confirms cilium/ebpf is wired into the project.
func DependencyMarker() string {
	spec := ebpf.ProgramSpec{}
	return fmt.Sprintf("%T", spec)
}

// ProbeSmokeCheck verifies that core eBPF objects can be created.
func ProbeSmokeCheck() error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("probe smoke requires linux host")
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("probe smoke requires privileged execution")
	}

	probeMap, err := ebpf.NewMap(&ebpf.MapSpec{
		Name:       "llm_slo_smoke",
		Type:       ebpf.Hash,
		KeySize:    4,
		ValueSize:  4,
		MaxEntries: 1,
	})
	if err != nil {
		return fmt.Errorf("create smoke map: %w", err)
	}
	defer probeMap.Close()

	return nil
}
