.PHONY: build test lint bench replay inject collector-smoke baseline-report

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

bench:
	go run ./cmd/benchgen --out artifacts/benchmarks

replay:
	go run ./cmd/faultreplay --scenario mixed --count 30 --out artifacts/fault-replay/fault_samples.jsonl
	go run ./cmd/benchgen --out artifacts/benchmarks-replay --scenario mixed_faults --input artifacts/fault-replay/fault_samples.jsonl

inject:
	go run ./cmd/faultinject --scenario mixed --count 30 --out artifacts/fault-injection/raw_samples.jsonl

collector-smoke:
	go run ./cmd/faultinject --scenario mixed --count 12 --out artifacts/fault-injection/raw_samples.jsonl
	go run ./cmd/collector --input artifacts/fault-injection/raw_samples.jsonl --output jsonl --output-path artifacts/collector/slo-events.jsonl

baseline-report:
	go run ./cmd/faultinject --scenario mixed --count 24 --out artifacts/fault-injection/raw_samples.jsonl
	go run ./cmd/faultreplay --scenario mixed --count 24 --out artifacts/fault-replay/fault_samples.jsonl
	go run ./cmd/benchgen --out artifacts/benchmarks-baseline --scenario mixed_faults --input artifacts/fault-replay/fault_samples.jsonl
	cp artifacts/benchmarks-baseline/report.md docs/benchmarks/reports/baseline-attribution-latest.md
