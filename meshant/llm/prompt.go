// prompt.go provides LoadPromptTemplate and HashPromptTemplate for reading and
// hashing extraction prompt files.
package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// LoadPromptTemplate reads a prompt template file, capped at maxSourceBytes.
// An empty file is valid (returns ""); a missing or oversized file returns an error.
func LoadPromptTemplate(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open prompt template %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxSourceBytes)+1) // +1 to detect oversized files
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read prompt template %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: prompt template %q exceeds %d bytes", path, maxSourceBytes)
	}
	return string(data), nil
}

// HashPromptTemplate computes a SHA-256 hash of the prompt template file at
// path and returns the first 16 hex characters.
//
// Returns "", nil for an empty path — valid for assist sessions that carry no
// template. Returns an error if the file cannot be read or exceeds
// maxSourceBytes (same limit as LoadPromptTemplate).
//
// The hash is computed from raw file bytes — Unicode normalization and
// line-ending differences produce different hashes. This is intentional:
// the hash detects physical file change, not semantic equivalence.
func HashPromptTemplate(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open prompt template %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxSourceBytes)+1) // +1 to detect oversized files
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read prompt template %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: prompt template %q exceeds %d bytes", path, maxSourceBytes)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16], nil
}
