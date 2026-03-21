package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
)

// maxSpansBytes caps the spans file size at 4 MiB, consistent with other
// structured input caps (LoadPromptTemplate, loadCriterionFile).
const maxSpansBytes = 4 * 1024 * 1024

// defaultAssistPrompt is the bundled assist prompt template path (repository root-relative).
const defaultAssistPrompt = "data/prompts/assist_pass.md"

// defaultAssistModel is the model used when --model is not supplied.
const defaultAssistModel = "claude-sonnet-4-6"

// cmdAssist implements the "assist" subcommand.
//
// Reads a spans file, calls the LLM once per span, presents each candidate
// draft for accept/edit/skip/quit, and writes accepted drafts plus a
// SessionRecord. Session output defaults mirror cmdExtract.
//
// client may be nil; a real AnthropicClient is then constructed from env vars.
// in is the interactive input reader (os.Stdin in production, strings.Reader
// in tests).
func cmdAssist(w io.Writer, client llm.LLMClient, in io.Reader, args []string) error {
	fs := flag.NewFlagSet("assist", flag.ContinueOnError)

	var spansFile string
	fs.StringVar(&spansFile, "spans-file", "", "path to spans file (required)")

	var promptTemplate string
	fs.StringVar(&promptTemplate, "prompt-template", defaultAssistPrompt, "path to assist prompt template")

	var modelID string
	fs.StringVar(&modelID, "model", defaultAssistModel, "LLM model ID")

	var sourceDocRef string
	fs.StringVar(&sourceDocRef, "source-doc-ref", "", "document reference string for provenance (optional)")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to criterion JSON file (optional)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write TraceDraft JSON to file (default: stdout)")

	var sessionOutputPath string
	fs.StringVar(&sessionOutputPath, "session-output", "", "write SessionRecord JSON to file (see default rules)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if spansFile == "" {
		return fmt.Errorf("assist: --spans-file is required\n\nUsage: meshant assist --spans-file <path> [--prompt-template <path>] [--model <id>] [--source-doc-ref <ref>] [--criterion-file <path>] [--output <file>] [--session-output <file>]")
	}

	// Default session output path when not explicitly provided.
	if sessionOutputPath == "" {
		if outputPath != "" {
			sessionOutputPath = outputPath + ".session.json"
		} else {
			sessionOutputPath = "session_" + time.Now().UTC().Format("20060102T150405Z") + ".json"
		}
	}

	var criterionRef string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("assist: %w", err)
		}
		criterionRef = c.Name
	}

	spansData, err := readSpansFile(spansFile)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}
	spans, err := llm.ParseSpans(spansData)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}

	if client == nil {
		c, err := llm.NewAnthropicClient(modelID)
		if err != nil {
			return fmt.Errorf("assist: %w", err)
		}
		client = c
	}

	opts := llm.AssistOptions{
		ModelID:            modelID,
		InputPath:          spansFile,
		PromptTemplatePath: promptTemplate,
		CriterionRef:       criterionRef,
		SourceDocRef:       sourceDocRef,
		OutputPath:         outputPath,
	}

	drafts, rec, err := llm.RunAssistSession(context.Background(), client, spans, opts, in, w)

	// Always write the SessionRecord before returning — even on error.
	// A session-write failure after a successful session is a hard error:
	// the provenance record is lost.
	sessionWriteErr := writeSessionRecord(sessionOutputPath, rec)
	if err != nil {
		// Primary session error takes precedence; demote session-write failure
		// to a warning so the session error is not masked.
		if sessionWriteErr != nil {
			fmt.Fprintf(w, "assist: warning: could not write session record to %q: %v\n", sessionOutputPath, sessionWriteErr)
		}
		return fmt.Errorf("assist: %w", err)
	}
	if sessionWriteErr != nil {
		// Session succeeded but record lost — provenance is broken.
		return fmt.Errorf("assist: write session record: %w", sessionWriteErr)
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("assist: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("assist: encode output: %w", err)
	}

	// Print provenance summary to w (stdout), never to the output file.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("assist: %w", err)
	}

	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath); err != nil {
		return err
	}

	// Surface per-span errors after writing all output — exit code reflects
	// partial span failures even though session record and drafts are persisted.
	if rec.ErrorNote != "" {
		return fmt.Errorf("assist: session completed with span errors: %s", rec.ErrorNote)
	}
	return nil
}

// readSpansFile reads up to maxSpansBytes from the spans file at path.
func readSpansFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read spans file %q: %w", path, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, int64(maxSpansBytes)+1) // +1 detects oversized files
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read spans file %q: %w", path, err)
	}
	if len(data) > maxSpansBytes {
		return nil, fmt.Errorf("spans file %q exceeds %d bytes", path, maxSpansBytes)
	}
	return data, nil
}
