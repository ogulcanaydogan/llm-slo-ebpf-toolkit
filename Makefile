.PHONY: \
	build \
	test \
	lint \
	schema-validate \
	prereq-check \
	kind-up \
	kind-down \
	bootstrap-tools \
	observability-up \
	observability-down \
	kind-observability-smoke \
	rag-service \
	demo-up \
	ebpf-gen \
	ebpf-smoke \
	bench \
	replay \
	inject \
	collector-smoke \
	baseline-report \
	chaos-matrix \
	chaos-agent-otlp \
	correlation-gate

SCHEMA_FILES := \
	docs/contracts/v1/slo-event.schema.json \
	docs/contracts/v1/incident-attribution.schema.json \
	docs/contracts/v1alpha1/probe-event.schema.json \
	config/toolkit.schema.json

build:
	go build ./...

test:
	go test ./...

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint is not installed"; \
		exit 1; \
	fi
	golangci-lint run

schema-validate:
	@for schema in $(SCHEMA_FILES); do \
		echo "validating $$schema"; \
		jq -e . "$$schema" >/dev/null; \
	done

prereq-check:
	go run ./cmd/sloctl prereq check

kind-up:
	./deploy/kind/kind-up.sh

kind-down:
	./deploy/kind/kind-down.sh

bootstrap-tools:
	./deploy/kind/bootstrap-tools.sh

observability-up:
	kubectl apply -k deploy/observability

observability-down:
	kubectl delete -k deploy/observability --ignore-not-found=true

kind-observability-smoke:
	./test/integration-kind/observability-smoke.sh

rag-service:
	go run ./demo/rag-service

demo-up:
	kubectl apply -k demo/rag-service/k8s

ebpf-gen:
	cd ebpf/bpf2go && go generate ./...

ebpf-smoke:
	./scripts/ebpf-smoke.sh

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

chaos-matrix:
	./scripts/chaos/run_fault_matrix.sh

chaos-agent-otlp:
	./scripts/chaos/set_agent_mode.sh mixed otlp

correlation-gate:
	go run ./cmd/correlationeval \
		--input pkg/correlation/testdata/labeled_pairs.jsonl \
		--out artifacts/correlation/eval_summary.json \
		--predictions-out artifacts/correlation/predictions.csv \
		--window-ms 2000 \
		--threshold 0.7 \
		--min-precision 0.9 \
		--min-recall 0.85
