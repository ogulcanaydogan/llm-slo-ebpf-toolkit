.PHONY: build test lint bench replay

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
