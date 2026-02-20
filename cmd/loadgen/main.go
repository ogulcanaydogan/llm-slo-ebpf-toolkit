package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"
)

type requestEvent struct {
	Timestamp      time.Time `json:"timestamp"`
	Profile        string    `json:"profile"`
	RequestID      string    `json:"request_id"`
	TraceID        string    `json:"trace_id"`
	PromptClass    string    `json:"prompt_class"`
	RetrievalDocs  int       `json:"retrieval_docs"`
	TargetTokens   int       `json:"target_tokens"`
	ExpectedTTFTMs int       `json:"expected_ttft_ms"`
}

var version = "dev"

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Println(version)
		return
	}

	profile := flag.String("profile", "rag_mixed_20rps", "load profile")
	duration := flag.Int("duration-sec", 60, "generation duration in seconds")
	rps := flag.Int("rps", 20, "requests per second")
	seed := flag.Int64("seed", 42, "deterministic seed")
	out := flag.String("out", "artifacts/loadgen/requests.jsonl", "output JSONL path")
	flag.Parse()

	if *duration <= 0 || *rps <= 0 {
		fmt.Fprintln(os.Stderr, "duration-sec and rps must be > 0")
		os.Exit(1)
	}

	total := *duration * *rps
	rng := rand.New(rand.NewSource(*seed))
	start := time.Now().UTC()

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "create output directory failed: %v\n", err)
		os.Exit(1)
	}
	file, err := os.Create(*out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create output file failed: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for i := 0; i < total; i++ {
		event := requestEvent{
			Timestamp:      start.Add(time.Duration(i) * time.Second / time.Duration(*rps)),
			Profile:        *profile,
			RequestID:      fmt.Sprintf("req-%06d", i),
			TraceID:        fmt.Sprintf("trace-%06d", i),
			PromptClass:    samplePromptClass(*profile, rng),
			RetrievalDocs:  2 + rng.Intn(8),
			TargetTokens:   64 + rng.Intn(512),
			ExpectedTTFTMs: expectedTTFT(*profile, rng),
		}
		if err := encoder.Encode(event); err != nil {
			fmt.Fprintf(os.Stderr, "encode event failed: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("wrote %d requests to %s\n", total, *out)
}

func samplePromptClass(profile string, rng *rand.Rand) string {
	switch profile {
	case "chat_short":
		return "chat_short"
	case "rag_medium":
		return "rag_medium"
	default:
		classes := []string{"chat_short", "rag_medium", "context_long"}
		return classes[rng.Intn(len(classes))]
	}
}

func expectedTTFT(profile string, rng *rand.Rand) int {
	switch profile {
	case "chat_short":
		return 80 + rng.Intn(70)
	case "rag_medium":
		return 140 + rng.Intn(150)
	case "context_long":
		return 280 + rng.Intn(250)
	default:
		return 90 + rng.Intn(220)
	}
}
