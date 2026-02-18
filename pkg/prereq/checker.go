package prereq

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	severityBlocker = "blocker"
	severityWarning = "warning"
)

var kernelVersionPattern = regexp.MustCompile(`^(\d+)\.(\d+)`)

// CheckResult is one prerequisite evaluation row.
type CheckResult struct {
	Name        string `json:"name"`
	Pass        bool   `json:"pass"`
	Severity    string `json:"severity"`
	Current     string `json:"current"`
	Required    string `json:"required"`
	Remediation string `json:"remediation"`
}

// Report is the full prereq check result.
type Report struct {
	GeneratedAt   time.Time     `json:"generated_at"`
	HostOS        string        `json:"host_os"`
	HostArch      string        `json:"host_arch"`
	KernelRelease string        `json:"kernel_release"`
	Checks        []CheckResult `json:"checks"`
	Pass          bool          `json:"pass"`
}

// Snapshot captures host facts before evaluation.
type Snapshot struct {
	HostOS        string
	HostArch      string
	KernelRelease string
	HasBTF        bool
	HasKernelHdrs bool
	HasBPFTool    bool
	HasClang      bool
	HasKind       bool
	HasHelm       bool
	IsRoot        bool
}

// CollectSnapshot gathers host facts from the current process environment.
func CollectSnapshot() Snapshot {
	kernel, _ := kernelRelease()
	return Snapshot{
		HostOS:        runtime.GOOS,
		HostArch:      runtime.GOARCH,
		KernelRelease: kernel,
		HasBTF:        pathExists("/sys/kernel/btf/vmlinux"),
		HasKernelHdrs: pathExists(kernelHeadersPath(kernel)),
		HasBPFTool:    hasBinary("bpftool"),
		HasClang:      hasBinary("clang"),
		HasKind:       hasBinary("kind"),
		HasHelm:       hasBinary("helm"),
		IsRoot:        os.Geteuid() == 0,
	}
}

// Evaluate returns a report with pass/fail checks.
func Evaluate(snapshot Snapshot) Report {
	checks := []CheckResult{
		{
			Name:        "host_linux",
			Pass:        snapshot.HostOS == "linux",
			Severity:    severityBlocker,
			Current:     snapshot.HostOS,
			Required:    "linux",
			Remediation: "Run prereq and privileged eBPF tests on a Linux host (self-hosted runner for CI).",
		},
		buildKernelCheck(snapshot.KernelRelease),
		{
			Name:        "btf_available",
			Pass:        snapshot.HasBTF,
			Severity:    severityBlocker,
			Current:     boolLabel(snapshot.HasBTF),
			Required:    "true",
			Remediation: "Enable kernel BTF and ensure /sys/kernel/btf/vmlinux exists.",
		},
		{
			Name:        "kernel_headers",
			Pass:        snapshot.HasKernelHdrs,
			Severity:    severityWarning,
			Current:     boolLabel(snapshot.HasKernelHdrs),
			Required:    "true",
			Remediation: "Install kernel headers matching uname -r for local probe build workflows.",
		},
		{
			Name:        "bpftool_installed",
			Pass:        snapshot.HasBPFTool,
			Severity:    severityBlocker,
			Current:     boolLabel(snapshot.HasBPFTool),
			Required:    "true",
			Remediation: "Install bpftool on Linux runner/host used for CO-RE generation and smoke tests.",
		},
		{
			Name:        "clang_installed",
			Pass:        snapshot.HasClang,
			Severity:    severityWarning,
			Current:     boolLabel(snapshot.HasClang),
			Required:    "true",
			Remediation: "Install clang/llvm for BPF object compilation.",
		},
		{
			Name:        "privileged_execution",
			Pass:        snapshot.IsRoot,
			Severity:    severityBlocker,
			Current:     boolLabel(snapshot.IsRoot),
			Required:    "true",
			Remediation: "Run in privileged context (root or equivalent capability setup) for probe load tests.",
		},
		{
			Name:        "kind_installed",
			Pass:        snapshot.HasKind,
			Severity:    severityWarning,
			Current:     boolLabel(snapshot.HasKind),
			Required:    "true",
			Remediation: "Install kind to run local multi-node integration lab.",
		},
		{
			Name:        "helm_installed",
			Pass:        snapshot.HasHelm,
			Severity:    severityWarning,
			Current:     boolLabel(snapshot.HasHelm),
			Required:    "true",
			Remediation: "Install helm for optional chart-based deployment flows.",
		},
	}

	pass := true
	for _, check := range checks {
		if check.Severity == severityBlocker && !check.Pass {
			pass = false
			break
		}
	}

	return Report{
		GeneratedAt:   time.Now().UTC(),
		HostOS:        snapshot.HostOS,
		HostArch:      snapshot.HostArch,
		KernelRelease: snapshot.KernelRelease,
		Checks:        checks,
		Pass:          pass,
	}
}

// RunLocal executes prereq evaluation on current host.
func RunLocal() Report {
	return Evaluate(CollectSnapshot())
}

// StrictPass returns true only if all checks pass, including warnings.
func StrictPass(report Report) bool {
	for _, check := range report.Checks {
		if !check.Pass {
			return false
		}
	}
	return true
}

// MarshalJSON returns pretty JSON for external reporting.
func MarshalJSON(report Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

func buildKernelCheck(release string) CheckResult {
	major, minor, err := ParseKernelRelease(release)
	if err != nil {
		return CheckResult{
			Name:        "kernel_version",
			Pass:        false,
			Severity:    severityBlocker,
			Current:     release,
			Required:    ">=5.15",
			Remediation: "Use Linux kernel 5.15+ for supported CO-RE signal set.",
		}
	}

	pass := major > 5 || (major == 5 && minor >= 15)
	return CheckResult{
		Name:        "kernel_version",
		Pass:        pass,
		Severity:    severityBlocker,
		Current:     fmt.Sprintf("%d.%d", major, minor),
		Required:    ">=5.15",
		Remediation: "Upgrade Linux kernel to >=5.15 on validation hosts.",
	}
}

// ParseKernelRelease extracts major/minor from kernel release strings like "6.8.0-31-generic".
func ParseKernelRelease(release string) (int, int, error) {
	match := kernelVersionPattern.FindStringSubmatch(strings.TrimSpace(release))
	if len(match) != 3 {
		return 0, 0, fmt.Errorf("unrecognized kernel release %q", release)
	}

	var major, minor int
	if _, err := fmt.Sscanf(match[0], "%d.%d", &major, &minor); err != nil {
		return 0, 0, fmt.Errorf("parse kernel release %q: %w", release, err)
	}
	return major, minor, nil
}

func kernelRelease() (string, error) {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func kernelHeadersPath(kernel string) string {
	if kernel == "" {
		return ""
	}
	return fmt.Sprintf("/lib/modules/%s/build", kernel)
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func boolLabel(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
