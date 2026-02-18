package safety

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// CPUSample stores process and host aggregate CPU counters.
type CPUSample struct {
	ProcessTicks uint64
	TotalTicks   uint64
}

// CPUSampler returns cumulative CPU counters.
type CPUSampler interface {
	Sample() (CPUSample, error)
}

// ProcCPUSampler reads Linux /proc counters for current process.
type ProcCPUSampler struct {
	PID int
}

// Sample reads process and host CPU counters.
func (s ProcCPUSampler) Sample() (CPUSample, error) {
	if runtime.GOOS != "linux" {
		return CPUSample{}, fmt.Errorf("cpu sampler requires linux")
	}
	pid := s.PID
	if pid <= 0 {
		pid = os.Getpid()
	}

	processTicks, err := readProcessTicks(pid)
	if err != nil {
		return CPUSample{}, err
	}
	totalTicks, err := readTotalTicks()
	if err != nil {
		return CPUSample{}, err
	}

	return CPUSample{
		ProcessTicks: processTicks,
		TotalTicks:   totalTicks,
	}, nil
}

// OverheadGuard computes process CPU overhead against total host CPU.
type OverheadGuard struct {
	maxPct float64
	prev   *CPUSample
	source CPUSampler
}

// NewOverheadGuard creates a guard using /proc sampling.
func NewOverheadGuard(maxPct float64) *OverheadGuard {
	return &OverheadGuard{
		maxPct: maxPct,
		source: ProcCPUSampler{PID: os.Getpid()},
	}
}

// NewOverheadGuardWithSampler creates a guard for tests.
func NewOverheadGuardWithSampler(maxPct float64, source CPUSampler) *OverheadGuard {
	return &OverheadGuard{
		maxPct: maxPct,
		source: source,
	}
}

// Evaluate returns current estimated process CPU percentage and trigger verdict.
func (g *OverheadGuard) Evaluate() (float64, bool, error) {
	if g.source == nil {
		return 0, false, fmt.Errorf("cpu sampler is nil")
	}
	sample, err := g.source.Sample()
	if err != nil {
		return 0, false, err
	}

	if g.prev == nil {
		g.prev = &sample
		return 0, false, nil
	}

	prev := g.prev
	g.prev = &sample
	if sample.TotalTicks <= prev.TotalTicks {
		return 0, false, nil
	}

	deltaProc := sample.ProcessTicks - prev.ProcessTicks
	deltaTotal := sample.TotalTicks - prev.TotalTicks
	if deltaTotal == 0 {
		return 0, false, nil
	}

	pct := (float64(deltaProc) / float64(deltaTotal)) * 100.0 * float64(runtime.NumCPU())
	if pct < 0 {
		pct = 0
	}
	return pct, pct > g.maxPct, nil
}

func readProcessTicks(pid int) (uint64, error) {
	path := fmt.Sprintf("/proc/%d/stat", pid)
	line, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	parts := strings.Fields(string(line))
	// utime is field 14 and stime is field 15 (1-based indexing).
	if len(parts) < 15 {
		return 0, fmt.Errorf("unexpected stat field count in %s", path)
	}
	utime, err := strconv.ParseUint(parts[13], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse utime: %w", err)
	}
	stime, err := strconv.ParseUint(parts[14], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse stime: %w", err)
	}
	return utime + stime, nil
}

func readTotalTicks() (uint64, error) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, fmt.Errorf("read /proc/stat: empty")
	}
	line := scanner.Text()
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, fmt.Errorf("unexpected cpu header in /proc/stat")
	}

	var total uint64
	for _, field := range fields[1:] {
		v, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse /proc/stat field: %w", err)
		}
		total += v
	}
	return total, nil
}
