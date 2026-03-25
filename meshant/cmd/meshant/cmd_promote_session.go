package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// maxSessionBytes caps the size of a session JSON file read by cmdPromoteSession.
// Session files are small (metadata only); 1 MiB is generous.
const maxSessionBytes = 1 * 1024 * 1024

// cmdPromoteSession implements the "promote-session" subcommand.
//
// Reads a SessionRecord JSON file from disk, promotes it to a canonical
// schema.Trace via llm.PromoteSession, and writes a single-element
// []schema.Trace JSON array to --output (or stdout).
//
// This closes the Principle 8 reflexivity gap: the LLM session — an
// observation act — enters the trace graph as a first-class record.
// The observer position is required; it names who is recording the session
// as an observation, not who ran the session.
func cmdPromoteSession(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("promote-session", flag.ContinueOnError)

	var sessionFile string
	fs.StringVar(&sessionFile, "session-file", "", "path to SessionRecord JSON file (required)")

	var observer string
	fs.StringVar(&observer, "observer", "", "observer position for the promoted trace (required)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write []Trace JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if sessionFile == "" {
		return fmt.Errorf("promote-session: --session-file is required\n\nUsage: meshant promote-session --session-file <path> --observer <position> [--output <file>]")
	}
	if observer == "" {
		return fmt.Errorf("promote-session: --observer is required — name the position from which this session is being recorded as a trace")
	}

	rec, err := readSessionFile(sessionFile)
	if err != nil {
		return fmt.Errorf("promote-session: %w", err)
	}

	tr, err := llm.PromoteSession(rec, observer)
	if err != nil {
		return fmt.Errorf("promote-session: %w", err)
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("promote-session: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode([]schema.Trace{tr}); err != nil {
		return fmt.Errorf("promote-session: encode output: %w", err)
	}

	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "promoted session %s to trace\n", rec.ID)
	return err
}

// readSessionFile reads and decodes a SessionRecord from the JSON file at path.
// Fails if the file is unreadable or does not contain valid SessionRecord JSON.
// Reads are capped at maxSessionBytes to prevent memory exhaustion.
func readSessionFile(path string) (llm.SessionRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return llm.SessionRecord{}, fmt.Errorf("cannot open session file %q: %w", path, err)
	}
	defer f.Close()

	var buf bytes.Buffer
	limited := io.LimitReader(f, maxSessionBytes+1) // +1 to detect oversized files
	if _, err := buf.ReadFrom(limited); err != nil {
		return llm.SessionRecord{}, fmt.Errorf("read session file %q: %w", path, err)
	}
	if buf.Len() > int(maxSessionBytes) {
		return llm.SessionRecord{}, fmt.Errorf("session file %q exceeds %d bytes", path, maxSessionBytes)
	}

	// SessionRecord is framework-written and will gain fields as the LLM pipeline
	// evolves. DisallowUnknownFields is NOT used here — unlike EquivalenceCriterion
	// (a human-authored declaration), a session file produced by a newer binary
	// must remain readable by older binaries. Unknown fields are silently ignored.
	var rec llm.SessionRecord
	if err := json.NewDecoder(&buf).Decode(&rec); err != nil {
		return llm.SessionRecord{}, fmt.Errorf("decode session file %q: %w", path, err)
	}
	return rec, nil
}
