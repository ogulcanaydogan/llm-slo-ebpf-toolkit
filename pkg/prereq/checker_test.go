package prereq

import "testing"

func TestParseKernelRelease(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		major   int
		minor   int
		wantErr bool
	}{
		{name: "ubuntu style", input: "6.8.0-31-generic", major: 6, minor: 8},
		{name: "kind style", input: "5.15.0", major: 5, minor: 15},
		{name: "arch style", input: "6.6.12-arch1-1", major: 6, minor: 6},
		{name: "invalid", input: "darwin", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			major, minor, err := ParseKernelRelease(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if major != tt.major || minor != tt.minor {
				t.Fatalf("expected %d.%d got %d.%d", tt.major, tt.minor, major, minor)
			}
		})
	}
}

func TestEvaluateBlockers(t *testing.T) {
	t.Parallel()

	report := Evaluate(Snapshot{
		HostOS:        "darwin",
		HostArch:      "arm64",
		KernelRelease: "23.5.0",
		HasBTF:        false,
		HasKernelHdrs: false,
		HasBPFTool:    false,
		HasClang:      true,
		HasKind:       true,
		HasHelm:       false,
		IsRoot:        false,
	})

	if report.Pass {
		t.Fatalf("expected report to fail with blocker checks")
	}
}

func TestStrictPass(t *testing.T) {
	t.Parallel()

	report := Report{
		Checks: []CheckResult{
			{Name: "blocker_ok", Pass: true, Severity: severityBlocker},
			{Name: "warning_failed", Pass: false, Severity: severityWarning},
		},
		Pass: true,
	}

	if StrictPass(report) {
		t.Fatalf("strict pass should fail when warning check fails")
	}
}
