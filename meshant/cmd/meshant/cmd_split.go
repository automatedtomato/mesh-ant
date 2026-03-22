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
)

// defaultSplitPrompt is the path to the bundled split prompt template,
// relative to the repository root. Used when --prompt-template is not supplied.
const defaultSplitPrompt = "data/prompts/split_pass.md"

// defaultSplitModel is the model ID used when --model is not supplied.
const defaultSplitModel = "claude-sonnet-4-6"

// cmdSplit implements the "split" subcommand.
//
// Calls the LLM to split a source document into observation spans — segments
// suitable for the "assist" command. Writes spans as a JSON array to --output
// (or stdout) and a SessionRecord to --session-output.
//
// No --criterion-file: split is boundary detection only, not analytical
// classification. The LLM is a mediating instrument proposing candidate
// boundaries; it does not apply equivalence criteria.
//
// client may be nil; a real AnthropicClient is then constructed from env vars.
// Tests inject a mock client.
func cmdSplit(w io.Writer, client llm.LLMClient, args []string) error {
	fs := flag.NewFlagSet("split", flag.ContinueOnError)

	var sourceDoc string
	fs.StringVar(&sourceDoc, "source-doc", "", "path to source document (required)")

	var sourceDocRef string
	fs.StringVar(&sourceDocRef, "source-doc-ref", "", "document reference string for provenance (defaults to --source-doc path)")

	var promptTemplate string
	fs.StringVar(&promptTemplate, "prompt-template", defaultSplitPrompt, "path to split prompt template")

	var modelID string
	fs.StringVar(&modelID, "model", defaultSplitModel, "LLM model ID")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write spans JSON array to file (default: stdout)")

	var sessionOutputPath string
	fs.StringVar(&sessionOutputPath, "session-output", "", "write SessionRecord JSON to file (see default rules)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if sourceDoc == "" {
		return fmt.Errorf("split: --source-doc is required\n\nUsage: meshant split --source-doc <path> [--source-doc-ref <ref>] [--prompt-template <path>] [--model <id>] [--output <file>] [--session-output <file>]")
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

	if client == nil {
		c, err := llm.NewAnthropicClient(modelID)
		if err != nil {
			return fmt.Errorf("split: %w", err)
		}
		client = c
	}

	opts := llm.SplitOptions{
		ModelID:            modelID,
		InputPath:          sourceDoc,
		PromptTemplatePath: promptTemplate,
		SourceDocRef:       sourceDocRef,
		OutputPath:         outputPath,
	}

	spans, rec, err := llm.RunSplit(context.Background(), client, opts)

	// Always write the SessionRecord before returning — even on RunSplit error.
	// The session record carries ErrorNote so failures are inspectable from disk.
	sessionWriteErr := writeSessionRecord(sessionOutputPath, rec)
	if err != nil {
		// Primary split error takes precedence; demote session-write failure
		// to a warning so the split error is not masked.
		if sessionWriteErr != nil {
			fmt.Fprintf(w, "split: warning: could not write session record to %q: %v\n", sessionOutputPath, sessionWriteErr)
		}
		return fmt.Errorf("split: %w", err)
	}
	if sessionWriteErr != nil {
		// Split succeeded but session record lost — provenance is broken.
		return fmt.Errorf("split: write session record: %w", sessionWriteErr)
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("split: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spans); err != nil {
		return fmt.Errorf("split: encode output: %w", err)
	}

	// Print count summary to w (stdout), never to the output file.
	if _, err := fmt.Fprintf(w, "proposed %d candidate observation spans\n", len(spans)); err != nil {
		return fmt.Errorf("split: write summary: %w", err)
	}

	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath)
	return err
}
