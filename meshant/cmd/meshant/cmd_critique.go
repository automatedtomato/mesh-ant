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

// defaultCritiquePrompt is the path to the bundled critique prompt template,
// relative to the repository root. Used when --prompt-template is not supplied.
const defaultCritiquePrompt = "data/prompts/critique_pass.md"

// defaultCritiqueModel is the model ID used when --model is not supplied.
const defaultCritiqueModel = "claude-sonnet-4-6"

// maxDraftBytes caps the size of the input drafts file read by cmdCritique.
// 4 MiB is generous for a human-analyst-produced JSON array and consistent
// with the cap applied to other structured inputs. A file exceeding this cap
// produces a JSON parse error (unexpected EOF), not a distinct size error.
const maxDraftBytes = 4 * 1024 * 1024

// cmdCritique implements the "critique" subcommand.
//
// It reads existing TraceDraft records from an input file, sends each to the
// LLM with the critique prompt, and writes derived drafts with
// ExtractionStage "critiqued" and DerivedFrom linking to the original.
//
// The LLM client is injected via the client parameter. When client is nil,
// cmdCritique constructs a real AnthropicClient from the environment. Tests
// pass a mock client so the critique pipeline is exercised without live API calls.
//
// Session output defaulting (same rules as cmdExtract):
//   - if --session-output is provided: write to that path
//   - if --output is a file: write to <output>.session.json
//   - if output is stdout: session record is not written (user opted out)
//
// An error is returned when no critique drafts were produced and the input
// contained at least one draft — this distinguishes a total LLM failure from
// a legitimate empty input.
//
// Flags:
//   - --input <path>              path to TraceDraft JSON array file (required)
//   - --prompt-template <path>    path to critique prompt template (default: data/prompts/critique_pass.md)
//   - --model <id>                LLM model ID (default: claude-sonnet-4-6)
//   - --source-doc-ref <ref>      document reference string for provenance (optional)
//   - --criterion-file <path>     optional criterion JSON file (CriterionRef provenance)
//   - --output <file>             write critique TraceDraft JSON to file (default: stdout)
//   - --session-output <file>     write SessionRecord JSON to file (see defaulting above)
//   - --id <id>                   critique only the draft with this ID (optional)
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

	// Derive sessionOutputPath from outputPath when not explicitly provided.
	if sessionOutputPath == "" && outputPath != "" {
		sessionOutputPath = outputPath + ".session.json"
	}
	// When outputPath is empty (stdout mode), session record is not written
	// unless --session-output was explicitly provided. This is T4 from
	// plan_thread_f.md: a user-agency decision, not a design flaw.

	// Resolve criterion reference string from optional --criterion-file.
	var criterionRef string
	if criterionFile != "" {
		c, err := loadCriterionFile(criterionFile)
		if err != nil {
			return fmt.Errorf("critique: %w", err)
		}
		criterionRef = c.Name
	}

	// Read and parse input drafts file. Reads are capped at maxDraftBytes to
	// prevent unbounded memory allocation on unexpectedly large inputs.
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

	// Construct real LLM client from environment when none is injected.
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
		SessionOutputPath:  sessionOutputPath,
		DraftID:            draftID,
	}

	critiqueDrafts, rec, err := llm.RunCritique(context.Background(), client, inputDrafts, opts)

	// Always write the SessionRecord when a path is available — even on error.
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

	// If the input was non-empty but no critiques were produced, the LLM failed
	// on all drafts. Treat this as an error so the caller sees a clear signal.
	if len(inputDrafts) > 0 && len(critiqueDrafts) == 0 {
		return fmt.Errorf("critique: no critique drafts produced; see session record for per-draft errors")
	}

	// Write critique TraceDraft JSON array to output destination.
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

	// Print a provenance summary to w (always stdout).
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

