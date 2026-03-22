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
// Calls the LLM to produce TraceDraft records from a source document. Writes
// TraceDraft JSON to --output (or stdout) and a SessionRecord to
// --session-output (defaults: <output>.session.json for file output, or
// session_<timestamp>.json in cwd for stdout output).
//
// client may be nil; a real AnthropicClient is then constructed from env vars.
// Tests inject a mock client.
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

	if sourceDocRef == "" {
		sourceDocRef = sourceDoc
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
			return fmt.Errorf("extract: %w", err)
		}
		criterionRef = c.Name
	}

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
	}

	drafts, rec, err := llm.RunExtraction(context.Background(), client, opts)

	// Always write the SessionRecord before returning — even on extraction error.
	// The session record carries ErrorNote so failures are inspectable from disk.
	// A session-write failure after a successful extraction is a hard error:
	// the provenance record is lost.
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

	// Print provenance summary to w (stdout), never to the output file.
	summary := loader.SummariseDrafts(drafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath)
	return err
}

// writeSessionRecord serialises rec as indented JSON to path.
// Encodes to a buffer before creating the file to avoid leaving a partial file
// on serialisation error. Shared by cmdExtract, cmdAssist, and cmdCritique.
func writeSessionRecord(path string, rec llm.SessionRecord) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rec); err != nil {
		return fmt.Errorf("encode session record: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}
