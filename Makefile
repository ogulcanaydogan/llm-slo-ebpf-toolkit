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
	bench-multi \
	replay \
	inject \
	collector-smoke \
	baseline-report \
	chaos-matrix \
	chaos-agent-otlp \
	correlation-gate \
	m5-gate \
	m5-candidate-rebuild \
	m5-baseline-rebuild \
	helm-lint \
	helm-template \
	cdgate-smoke \
	runner-validate \
	runner-profile-discovery

SCHEMA_FILES := \
	docs/contracts/v1/slo-event.schema.json \
	docs/contracts/v1/incident-attribution.schema.json \
	docs/contracts/v1alpha1/probe-event.schema.json \
	config/toolkit.schema.json

M5_CANDIDATE_ROOT ?= artifacts/weekly-benchmark
M5_BASELINE_ROOT ?= artifacts/weekly-benchmark/baseline
M5_BASELINE_MANIFEST ?= artifacts/weekly-benchmark/baseline/manifest.json
M5_CANDIDATE_REF ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo local)
M5_CANDIDATE_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo local)
M5_REQUIRE_BASELINE_MANIFEST ?= false
comma := ,
M5_SCENARIOS ?= dns_latency,cpu_throttle,provider_throttle,memory_pressure,network_partition,mixed,mixed_multi
M5_SCENARIOS_LIST := $(subst $(comma), ,$(M5_SCENARIOS))
M5_MAX_OVERHEAD_PCT ?= 3
M5_MAX_VARIANCE_PCT ?= 10
M5_MIN_RUNS ?= 3
M5_TTFT_REGRESSION_PCT ?= 5
M5_ALPHA ?= 0.05
M5_BOOTSTRAP_ITERS ?= 1000
M5_MIN_SAMPLES ?= 30
M5_MIN_CLIFFS_DELTA ?= 0.147
M5_BASELINE_SAMPLE_COUNT ?= 36
M5_CANDIDATE_RUN_COUNT ?= 3
M5_CANDIDATE_SAMPLES_PER_RUN ?= 24
M5_OUT_JSON ?= artifacts/weekly-benchmark/m5_gate_summary.json
M5_OUT_MD ?= artifacts/weekly-benchmark/m5_gate_summary.md

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
	go run ./cmd/schemavalidate

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

m5-gate:
	go run ./cmd/m5gate \
		--candidate-root $(M5_CANDIDATE_ROOT) \
		--baseline-root $(M5_BASELINE_ROOT) \
		--baseline-manifest $(M5_BASELINE_MANIFEST) \
		--candidate-ref $(M5_CANDIDATE_REF) \
		--candidate-commit $(M5_CANDIDATE_COMMIT) \
		--require-baseline-manifest=$(M5_REQUIRE_BASELINE_MANIFEST) \
		--scenarios $(M5_SCENARIOS) \
		--max-overhead-pct $(M5_MAX_OVERHEAD_PCT) \
		--max-variance-pct $(M5_MAX_VARIANCE_PCT) \
		--min-runs $(M5_MIN_RUNS) \
		--ttft-regression-pct $(M5_TTFT_REGRESSION_PCT) \
		--alpha $(M5_ALPHA) \
		--bootstrap-iters $(M5_BOOTSTRAP_ITERS) \
		--min-samples $(M5_MIN_SAMPLES) \
		--min-cliffs-delta $(M5_MIN_CLIFFS_DELTA) \
		--out-json $(M5_OUT_JSON) \
		--out-md $(M5_OUT_MD)

m5-candidate-rebuild:
	@mkdir -p $(M5_CANDIDATE_ROOT)
	@for scenario in $(M5_SCENARIOS_LIST); do \
		for run in $$(seq 1 $(M5_CANDIDATE_RUN_COUNT)); do \
			run_dir="$(M5_CANDIDATE_ROOT)/$$scenario/run-$$run"; \
			mkdir -p "$$run_dir"; \
			echo "rebuilding candidate run $$run for $$scenario"; \
			go run ./cmd/faultinject --scenario "$$scenario" --count $(M5_CANDIDATE_SAMPLES_PER_RUN) --out "$$run_dir/raw_samples.jsonl"; \
			go run ./cmd/faultreplay --scenario "$$scenario" --count $(M5_CANDIDATE_SAMPLES_PER_RUN) --out "$$run_dir/replay.jsonl"; \
			bench_scenario="$$scenario"; \
			if [ "$$scenario" = "mixed" ]; then bench_scenario="mixed_faults"; fi; \
			go run ./cmd/benchgen --out "$$run_dir" --scenario "$$bench_scenario" --input "$$run_dir/replay.jsonl"; \
		done; \
	done

m5-baseline-rebuild:
	@mkdir -p $(M5_BASELINE_ROOT)
	@for scenario in $(M5_SCENARIOS_LIST); do \
		out_dir="$(M5_BASELINE_ROOT)/$$scenario"; \
		mkdir -p "$$out_dir"; \
		echo "rebuilding baseline samples for $$scenario -> $$out_dir/raw_samples.jsonl"; \
		go run ./cmd/faultinject --scenario "$$scenario" --count $(M5_BASELINE_SAMPLE_COUNT) --out "$$out_dir/raw_samples.jsonl"; \
	done
	@printf '{\n  "source_ref": "local-baseline-rebuild",\n  "source_commit": "%s",\n  "generated_at": "%s"\n}\n' "$$(git rev-parse HEAD 2>/dev/null || echo local)" "$$(date -u +%Y-%m-%dT%H:%M:%SZ)" > $(M5_BASELINE_MANIFEST)

runner-validate:
	gh api repos/$${GITHUB_REPOSITORY:-ogulcanaydogan/LLM-SLO-eBPF-Toolkit}/actions/runners \
		--jq '.runners[] | {name,status,busy,labels:[.labels[].name]}'

runner-profile-discovery:
	mkdir -p artifacts/compatibility
	RUNNER_STATUS_TOKEN="$${RUNNER_STATUS_TOKEN:-$${GITHUB_TOKEN:-$$(gh auth token 2>/dev/null || true)}}" \
	GITHUB_REPOSITORY="$${GITHUB_REPOSITORY:-ogulcanaydogan/LLM-SLO-eBPF-Toolkit}" \
	./scripts/ci/check_runner_profiles.sh \
		--profiles "$${RUNNER_PROFILES:-kernel-5-15,kernel-6-8}" \
		--out artifacts/compatibility/runner-discovery.local.json
	jq . artifacts/compatibility/runner-discovery.local.json

helm-lint:
	helm lint charts/llm-slo-agent

helm-template:
	helm template test-release charts/llm-slo-agent

bench-multi:
	go run ./cmd/faultreplay --scenario mixed_multi --count 30 --out artifacts/fault-replay/multi_fault_samples.jsonl
	go run ./cmd/benchgen --out artifacts/benchmarks-multi --scenario mixed_multi --input artifacts/fault-replay/multi_fault_samples.jsonl

cdgate-smoke:
	go test ./pkg/cdgate/... -v -count=1
