package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// ValidateAgainstSchema validates an arbitrary payload against a JSON schema file.
func ValidateAgainstSchema(schemaPath string, payload interface{}) error {
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("read schema %s: %w", schemaPath, err)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(schemaBytes),
		gojsonschema.NewBytesLoader(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}

	if result.Valid() {
		return nil
	}

	errors := make([]string, 0, len(result.Errors()))
	for _, issue := range result.Errors() {
		errors = append(errors, issue.String())
	}
	return fmt.Errorf("payload failed schema validation: %s", strings.Join(errors, "; "))
}
