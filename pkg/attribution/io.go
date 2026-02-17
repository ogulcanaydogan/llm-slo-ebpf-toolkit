package attribution

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LoadSamplesFromJSONL loads fault samples from a JSONL file.
func LoadSamplesFromJSONL(path string) ([]FaultSample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open samples file: %w", err)
	}
	defer file.Close()

	samples := make([]FaultSample, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var sample FaultSample
		if err := json.Unmarshal([]byte(line), &sample); err != nil {
			return nil, fmt.Errorf("parse sample: %w", err)
		}
		samples = append(samples, sample)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan samples file: %w", err)
	}
	if len(samples) == 0 {
		return nil, fmt.Errorf("no samples loaded from %s", path)
	}
	return samples, nil
}
