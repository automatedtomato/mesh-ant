package main

import (
	"bytes"
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

// defaultExtractionPrompt is the path to the bundled extraction prompt
// template, relative to the repository root. It is used when --prompt-template
// is not supplied.
const defaultExtractionPrompt = "data/prompts/extraction_pass.md"

// defaultExtractModel is the model ID used when --model is not supplied.
const defaultExtractModel = "claude-sonnet-4-6"

// cmdExtract implements the "extract" subcommand.
//
// It calls the LLM to produce TraceDraft records from a source document,
// then writes two outputs:
//  1. TraceDraft JSON array to --output (or stdout)
//  2. SessionRecord JSON to --session-output (see defaulting rules below)
//
// The LLM client is injected via the client parameter. When client is nil,
// cmdExtract constructs a real AnthropicClient from the environment (reading
// MESHANT_LLM_API_KEY or ANTHROPIC_API_KEY). Tests pass a mock client so the
// extraction pipeline is exercised without live API calls.
//
// Session output defaulting:
//   - if --session-output is provided: write to that path
//   - if --output is a file: write to <output>.session.json
//   - if output is stdout: write to session_<compact-ISO8601-timestamp>.json in cwd
//
// Flags:
//   - --source-doc <path>         path to source document (required)
//   - --source-doc-ref <ref>      document reference string for provenance (defaults to --source-doc path)
//   - --prompt-template <path>    path to extraction prompt template (default: data/prompts/extraction_pass.md)
//   - --model <id>                LLM model ID (default: claude-sonnet-4-6)
//   - --criterion-file <path>     optional criterion JSON file (CriterionRef provenance)
//   - --output <file>             write TraceDraft JSON to file (default: stdout)
//   - --session-output <file>     write SessionRecord JSON to file (see defaulting above)
func cmdExtract(w io.Writer, client llm.LLMClient, args []string) error {
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)

	var sourceDoc string
	fs.StringVar(&sourceDoc, "source-doc", "", "path to source document (required)")

	var sourceDocRef string
	fs.StringVar(&sourceDocRef, "source-doc-ref", "", "document reference string for provenance (defaults to --source-doc path)")

	var promptTemplate string
	fs.StringVar(&promptTemplate, "prompt-template", defaultExtractionPrompt, "path to extraction prompt template")

	var modelID string
	fs.StringVar(&modelID, "model", defaultExtractModel, "LLM model ID")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to criterion JSON file (optional)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write TraceDraft JSON to file (default: stdout)")

	var sessionOutputPath string
	fs.StringVar(&sessionOutputPath, "session-output", "", "write SessionRecord JSON to file (see default rules)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if sourceDoc == "" {
		return fmt.Errorf("extract: --source-doc is required\n\nUsage: meshant extract --source-doc <path> [--source-doc-ref <ref>] [--prompt-template <path>] [--model <id>] [--criterion-file <path>] [--output <file>] [--session-output <file>]")
	}

	// sourceDocRef defaults to the source document path when not provided.
	if sourceDocRef == "" {
		sourceDocRef = sourceDoc
	}

	// Derive sessionOutputPath from outputPath when not explicitly provided.
	// For a file output, use <output>.session.json.
	// For stdout, use session_<timestamp>.json in the current directory.
	if sessionOutputPath == "" {
		if outputPath != "" {
			sessionOutputPath = outputPath + ".session.json"
		} else {
			sessionOutputPath = "session_" + time.Now().UTC().Format("20060102T150405Z") + ".json"
		}
	}

	// Resolve criterion reference string from optional --criterion-file.
	var criterionRef string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}
		criterionRef = c.Name
	}

	// Construct real LLM client from environment when none is injected.
	if client == nil {
		c, err := llm.NewAnthropicClient(modelID)
		if err != nil {
			return fmt.Errorf("extract: %w", err)
		}
		client = c
	}

	opts := llm.ExtractionOptions{
		ModelID:            modelID,
		InputPath:          sourceDoc,
		PromptTemplatePath: promptTemplate,
		CriterionRef:       criterionRef,
		SourceDocRef:       sourceDocRef,
		OutputPath:         outputPath,
		SessionOutputPath:  sessionOutputPath,
	}

	drafts, rec, err := llm.RunExtraction(context.Background(), client, opts)

	// Always write the SessionRecord before returning — even on extraction error.
	// The session record carries ErrorNote so failures are inspectable from disk
	// without re-running. When the extraction succeeded but the session write
	// fails, that is a hard error: the provenance record is lost.
	sessionWriteErr := writeSessionRecord(sessionOutputPath, rec)
	if err != nil {
		// Primary extraction error takes precedence; demote session-write failure
		// to a warning so the extraction error is not masked.
		if sessionWriteErr != nil {
			fmt.Fprintf(w, "extract: warning: could not write session record to %q: %v\n", sessionOutputPath, sessionWriteErr)
		}
		return fmt.Errorf("extract: %w", err)
	}
	if sessionWriteErr != nil {
		// Extraction succeeded but session record lost — provenance is broken.
		return fmt.Errorf("extract: write session record: %w", sessionWriteErr)
	}

	// Write TraceDraft JSON array to output destination.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(drafts); err != nil {
		return fmt.Errorf("extract: encode output: %w", err)
	}

	// Print a provenance summary to w (always stdout) — never to the output
	// file. This mirrors cmdDraft's behaviour.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	// Confirm file output paths.
	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath)
	return err
}

// writeSessionRecord serialises rec as indented JSON to path.
// It encodes to a buffer before creating the file so that a serialisation
// failure does not leave an empty or truncated file on disk.
// Used only by cmdExtract; move to main.go when other commands need session persistence.
func writeSessionRecord(path string, rec llm.SessionRecord) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		return fmt.Errorf("encode session record: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
