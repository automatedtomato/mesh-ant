// shared.go contains helpers shared across llm package operations.
//
// readSourceDoc and isRefusal were originally in extract.go. Moving them here
// avoids duplication now that split.go also needs them. The move is purely
// structural — behaviour is unchanged.
package llm

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// maxSourceBytes caps source document size at 1 MiB to prevent unexpected token costs.
// Moved from types.go to shared.go so all llm operations can reference it.
const maxSourceBytes = 1 * 1024 * 1024

// readSourceDoc reads the source document at path, enforcing the maxSourceBytes cap.
// Returns the document as a string, or an error if the file cannot be read or
// exceeds the size limit.
func readSourceDoc(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open source doc %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxSourceBytes)+1) // +1 to detect oversized files
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read source doc %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: source doc %q exceeds %d bytes", path, maxSourceBytes)
	}
	return string(data), nil
}

// isRefusal reports whether the LLM response looks like an explicit refusal.
// Conservative heuristic — undetected refusals fall through to the
// malformed-output path rather than producing a false positive.
func isRefusal(response string) bool {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return false // empty → malformed, not refusal
	}
	prefixes := []string{
		"I cannot",
		"I'm sorry",
		"I am sorry",
		"I apologize",
		"I'm unable",
		"I am unable",
	}
	lower := strings.ToLower(trimmed)
	for _, p := range prefixes {
		if strings.HasPrefix(lower, strings.ToLower(p)) {
			return true
		}
	}
	return false
}
