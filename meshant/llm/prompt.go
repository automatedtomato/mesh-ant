// prompt.go provides LoadPromptTemplate for reading extraction prompt files.
package llm

import (
	"fmt"
	"io"
	"os"
)

// LoadPromptTemplate reads a prompt template file from disk and returns its
// contents as a string. The file is read up to maxSourceBytes; files larger
// than that are rejected with a clear error.
//
// An empty file returns an empty string without error — empty system
// instructions are valid (the caller may supply instructions another way).
// A missing file returns a clear error naming the path.
func LoadPromptTemplate(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("llm: open prompt template %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxSourceBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("llm: read prompt template %q: %w", path, err)
	}
	if len(data) > maxSourceBytes {
		return "", fmt.Errorf("llm: prompt template %q exceeds %d bytes", path, maxSourceBytes)
	}
	return string(data), nil
}
