package main

import (
	"math/rand"
	"reflect"
	"testing"
)

func TestGenerateTokensDeterministic(t *testing.T) {
	tokensA := generateTokens("Explain DNS impact on TTFT", 12, 42)
	tokensB := generateTokens("Explain DNS impact on TTFT", 12, 42)
	if !reflect.DeepEqual(tokensA, tokensB) {
		t.Fatalf("expected deterministic token generation")
	}
	if len(tokensA) != 12 {
		t.Fatalf("expected 12 tokens, got %d", len(tokensA))
	}
}

func TestPlanForRequestDeterministic(t *testing.T) {
	rngA := rand.New(rand.NewSource(99))
	rngB := rand.New(rand.NewSource(99))

	planA := planForRequest("rag_medium", "hello", rngA)
	planB := planForRequest("rag_medium", "hello", rngB)

	if planA != planB {
		t.Fatalf("expected deterministic plan output")
	}
	if planA.DNSMS <= 0 || planA.VectorDBMS <= 0 {
		t.Fatalf("expected positive retrieval components")
	}
}

func TestSelectDocsCount(t *testing.T) {
	docs := []corpusDoc{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}}
	rng := rand.New(rand.NewSource(7))
	picked := selectDocs(docs, 3, rng)
	if len(picked) != 3 {
		t.Fatalf("expected 3 docs, got %d", len(picked))
	}
}
