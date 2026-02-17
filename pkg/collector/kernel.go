package collector

import (
	"fmt"

	"github.com/cilium/ebpf"
)

// DependencyMarker confirms cilium/ebpf is wired into the project.
func DependencyMarker() string {
	spec := ebpf.ProgramSpec{}
	return fmt.Sprintf("%T", spec)
}
