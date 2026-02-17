.PHONY: build test lint bench

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

bench:
	go run ./cmd/benchgen --out artifacts/benchmarks
