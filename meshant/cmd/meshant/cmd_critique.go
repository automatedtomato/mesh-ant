package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/llm"
	"github.com/automatedtomato/mesh-ant/meshant/loader"
	"github.com/automatedtomato/mesh-ant/meshant/schema"
)

// defaultCritiquePrompt is the bundled critique prompt template path (repository root-relative).
const defaultCritiquePrompt = "data/prompts/critique_pass.md"

// defaultCritiqueModel is the model used when --model is not supplied.
const defaultCritiqueModel = "claude-sonnet-4-6"

// maxDraftBytes caps the input drafts file at 4 MiB, consistent with other
// structured input caps.
const maxDraftBytes = 4 * 1024 * 1024

// cmdCritique implements the "critique" subcommand.
//
// Reads TraceDraft records from an input file, sends each to the LLM, and
// writes derived drafts with ExtractionStage "critiqued" and DerivedFrom
// linking to the original. Session output defaults: <output>.session.json for
// file output; no session record when writing to stdout (user opted out).
//
// Returns an error when no critique drafts were produced but the input was
// non-empty — this distinguishes a total LLM failure from an empty input.
//
// client may be nil; a real AnthropicClient is then constructed from env vars.
// Tests inject a mock client.
func cmdCritique(w io.Writer, client llm.LLMClient, args []string) error {
	fs := flag.NewFlagSet("critique", flag.ContinueOnError)

	var inputPath string
	fs.StringVar(&inputPath, "input", "", "path to TraceDraft JSON array file (required)")

	var promptTemplate string
	fs.StringVar(&promptTemplate, "prompt-template", defaultCritiquePrompt, "path to critique prompt template")

	var modelID string
	fs.StringVar(&modelID, "model", defaultCritiqueModel, "LLM model ID")

	var sourceDocRef string
	fs.StringVar(&sourceDocRef, "source-doc-ref", "", "document reference string for provenance")

	var criterionFile string
	fs.StringVar(&criterionFile, "criterion-file", "", "path to criterion JSON file (optional)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write critique TraceDraft JSON to file (default: stdout)")

	var sessionOutputPath string
	fs.StringVar(&sessionOutputPath, "session-output", "", "write SessionRecord JSON to file (when --output is a file, defaults to <output>.session.json; when stdout, no record is written unless this flag is set)")

	var draftID string
	fs.StringVar(&draftID, "id", "", "critique only the draft with this ID (optional)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if inputPath == "" {
		return fmt.Errorf("critique: --input is required\n\nUsage: meshant critique --input <path> [--prompt-template <path>] [--model <id>] [--source-doc-ref <ref>] [--criterion-file <path>] [--output <file>] [--session-output <file>] [--id <id>]")
	}

	// Default session output path; no session record when writing to stdout
	// unless --session-output was explicitly provided (user-agency, not a flaw).
	if sessionOutputPath == "" && outputPath != "" {
		sessionOutputPath = outputPath + ".session.json"
	}

	var criterionRef string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("critique: %w", err)
		}
		criterionRef = c.Name
	}

	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("critique: read input %q: %w", inputPath, err)
	}
	defer f.Close()

	limited := io.LimitReader(f, maxDraftBytes)
	dec := json.NewDecoder(limited)
	var inputDrafts []schema.TraceDraft
	if err := dec.Decode(&inputDrafts); err != nil {
		return fmt.Errorf("critique: parse input %q: %w", inputPath, err)
	}

	if client == nil {
		c, err := llm.NewAnthropicClient(modelID)
		if err != nil {
			return fmt.Errorf("critique: %w", err)
		}
		client = c
	}

	opts := llm.CritiqueOptions{
		ModelID:            modelID,
		InputPath:          inputPath,
		PromptTemplatePath: promptTemplate,
		CriterionRef:       criterionRef,
		SourceDocRef:       sourceDocRef,
		OutputPath:         outputPath,
		DraftID:            draftID,
	}

	critiqueDrafts, rec, err := llm.RunCritique(context.Background(), client, inputDrafts, opts)

	// Always write the SessionRecord before returning — even on error.
	if sessionOutputPath != "" {
		sessionWriteErr := writeSessionRecord(sessionOutputPath, rec)
		if sessionWriteErr != nil {
			if err != nil {
				fmt.Fprintf(w, "critique: warning: could not write session record to %q: %v\n", sessionOutputPath, sessionWriteErr)
			} else {
				return fmt.Errorf("critique: write session record: %w", sessionWriteErr)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("critique: %w", err)
	}

	if len(inputDrafts) > 0 && len(critiqueDrafts) == 0 {
		return fmt.Errorf("critique: no critique drafts produced; see session record for per-draft errors")
	}

	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("critique: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	enc := json.NewEncoder(dest)
	enc.SetIndent("", "  ")
	if err := enc.Encode(critiqueDrafts); err != nil {
		return fmt.Errorf("critique: encode output: %w", err)
	}

	// Print provenance summary to w (stdout), never to the output file.
	summary := loader.SummariseDrafts(critiqueDrafts)
	if err := loader.PrintDraftSummary(w, summary); err != nil {
		return fmt.Errorf("critique: %w", err)
	}

	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	if sessionOutputPath != "" {
		_, err = fmt.Fprintf(w, "wrote session record to %s\n", sessionOutputPath)
		return err
	}
	return nil
}

