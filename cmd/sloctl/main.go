package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/prereq"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "prereq":
		runPrereq(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		printUsage()
		os.Exit(2)
	}
}

func runPrereq(args []string) {
	if len(args) == 0 {
		printPrereqUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "check":
		runPrereqCheck(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown prereq subcommand %q\n", args[0])
		printPrereqUsage()
		os.Exit(2)
	}
}

func runPrereqCheck(args []string) {
	fs := flag.NewFlagSet("sloctl prereq check", flag.ExitOnError)
	output := fs.String("output", "text", "output mode: text|json")
	strict := fs.Bool("strict", false, "treat warnings as failures")
	_ = fs.Parse(args)

	report := prereq.RunLocal()

	switch *output {
	case "json":
		payload, err := prereq.MarshalJSON(report)
		if err != nil {
			fmt.Fprintf(os.Stderr, "marshal report: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(payload))
	case "text":
		printTextReport(report)
	default:
		fmt.Fprintf(os.Stderr, "unsupported output mode %q\n", *output)
		os.Exit(2)
	}

	pass := report.Pass
	if *strict {
		pass = prereq.StrictPass(report)
	}
	if !pass {
		os.Exit(1)
	}
}

func printTextReport(report prereq.Report) {
	fmt.Printf("generated_at: %s\n", report.GeneratedAt.Format("2006-01-02T15:04:05Z07:00"))
	fmt.Printf("host: %s/%s\n", report.HostOS, report.HostArch)
	fmt.Printf("kernel_release: %s\n", emptyFallback(report.KernelRelease, "unknown"))
	fmt.Println()
	fmt.Println("checks:")
	for _, check := range report.Checks {
		status := "PASS"
		if !check.Pass {
			status = "FAIL"
		}
		fmt.Printf("- [%s] (%s) %s\n", status, strings.ToUpper(check.Severity), check.Name)
		fmt.Printf("  current: %s\n", emptyFallback(check.Current, "n/a"))
		fmt.Printf("  required: %s\n", emptyFallback(check.Required, "n/a"))
		fmt.Printf("  remediation: %s\n", emptyFallback(check.Remediation, "n/a"))
	}
	fmt.Println()
	if report.Pass {
		fmt.Println("result: PASS (all blocker checks satisfied)")
		return
	}
	fmt.Println("result: FAIL (one or more blocker checks failed)")
}

func emptyFallback(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  sloctl prereq check [--output text|json] [--strict]")
}

func printPrereqUsage() {
	fmt.Println("Usage:")
	fmt.Println("  sloctl prereq check [--output text|json] [--strict]")
}
