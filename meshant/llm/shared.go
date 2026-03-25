// shared.go contains helpers shared across llm package operations.
//
// readSourceDoc and isRefusal were originally in extract.go. Moving them here
// avoids duplication now that split.go also needs them. The move is purely
// structural — behaviour is unchanged.
//
// stampProvenance, validateIntentionallyBlank, splitErrNotes, and joinErrNotes
// were previously scattered across extract.go, assist.go, and critique.go.
// Consolidated here to provide a single point of maintenance for F.1 provenance
// conventions and error-note accumulation.
package llm

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
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

// stripPreamble trims any leading text before the first '[' and any trailing
// text after the last ']' from s. It returns the trimmed string and true if
// an opening bracket was found, or the original string and false otherwise.
// Callers use the boolean to decide whether to return an error or proceed —
// parseResponse silently passes through while parseSplitResponse errors on false.
func stripPreamble(s string) (trimmed string, found bool) {
	idx := strings.Index(s, "[")
	if idx < 0 {
		return s, false
	}
	s = s[idx:]
	if end := strings.LastIndex(s, "]"); end >= 0 {
		s = s[:end+1]
	}
	return s, true
}

// filterBlanks returns only the non-blank (after TrimSpace) strings from src.
// Used by ParseSpans (assist.go) and parseSplitResponse (split.go) to drop
// empty entries from LLM-produced string slices.
func filterBlanks(src []string) []string {
	var result []string
	for _, s := range src {
		if strings.TrimSpace(s) != "" {
			result = append(result, s)
		}
	}
	return result
}

// stampProvenance stamps the framework-assigned F.1 provenance fields on d and
// generates its ID. stage is caller-supplied: extract/assist pass "weak-draft";
// critique passes "" (RunCritique sets ExtractionStage after the call).
//
// Extracted from the identical blocks previously duplicated across extractSingleDoc
// (extract.go), parseSingleDraft (assist.go), and parseCritiqueDraft (critique.go).
func stampProvenance(d *schema.TraceDraft, now time.Time, modelID, sessionID, sourceDocRef, stage string) error {
	id, err := loader.NewUUID()
	if err != nil {
		return fmt.Errorf("generate UUID: %w", err)
	}
	d.ID = id
	d.Timestamp = now
	d.ExtractedBy = modelID     // D2
	d.ExtractionStage = stage   // D4
	d.SessionRef = sessionID    // F.0
	d.SourceDocRef = sourceDocRef

	// Append framework uncertainty note (D3); preserve any LLM-set note.
	if d.UncertaintyNote != "" {
		d.UncertaintyNote = d.UncertaintyNote + " " + frameworkUncertaintyNote
	} else {
		d.UncertaintyNote = frameworkUncertaintyNote
	}
	return nil
}

// validateIntentionallyBlank returns an error if any name is not a known content
// field — provenance fields cannot be declared blank by the LLM (D7).
// Moved here from extract.go so assist.go and critique.go share a single copy.
func validateIntentionallyBlank(fields []string) error {
	for _, name := range fields {
		if !knownContentFields[name] {
			return fmt.Errorf("intentionally_blank: %q is not a valid content field name", name)
		}
	}
	return nil
}

// splitErrNotes splits a semicolon-separated ErrorNote into individual notes.
// Moved here from assist.go so critique.go can share the same accumulation logic.
func splitErrNotes(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "; ")
}

// joinErrNotes joins error notes with "; " for storage in ErrorNote.
func joinErrNotes(notes []string) string {
	return strings.Join(notes, "; ")
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
