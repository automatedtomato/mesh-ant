package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/automatedtomato/mesh-ant/meshant/adapter"
)

// cmdConvert implements the "convert" subcommand.
//
// Converts a non-text source file (PDF, HTML, JSON/log) to plain text and
// writes it to stdout or a file. This is a two-step workflow entry point:
// analysts convert first, inspect the extracted text, and then run extract.
// The conversion step is a mediating act — the adapter name is reported so
// the transformation is visible, not hidden.
//
// Usage:
//
//	meshant convert --adapter <name> --source-doc <path> [--output <file>]
//
// Known adapter names: pdf, html, jsonlog.
func cmdConvert(w io.Writer, args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)

	var adapterName string
	fs.StringVar(&adapterName, "adapter", "", "adapter name: pdf, html, or jsonlog (required)")

	var sourceDoc string
	fs.StringVar(&sourceDoc, "source-doc", "", "path to source document (required)")

	var outputPath string
	fs.StringVar(&outputPath, "output", "", "write converted text to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if adapterName == "" {
		return fmt.Errorf("convert: --adapter is required\n\nUsage: meshant convert --adapter <pdf|html|jsonlog> --source-doc <path> [--output <file>]")
	}
	if sourceDoc == "" {
		return fmt.Errorf("convert: --source-doc is required\n\nUsage: meshant convert --adapter <pdf|html|jsonlog> --source-doc <path> [--output <file>]")
	}

	a, err := adapter.ForName(adapterName)
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}

	result, err := a.Convert(sourceDoc)
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}

	// Write the extracted text to the output destination.
	dest, err := outputWriter(w, outputPath)
	if err != nil {
		return fmt.Errorf("convert: %w", err)
	}
	if f, ok := dest.(*os.File); ok {
		defer f.Close()
	}

	if _, err := fmt.Fprint(dest, result.Text); err != nil {
		return fmt.Errorf("convert: write output: %w", err)
	}

	// Print confirmation and metadata to w (stdout), never to the output file.
	if err := confirmOutput(w, outputPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "adapter: %s", result.AdapterName); err != nil {
		return err
	}
	for k, v := range result.Metadata {
		if _, err := fmt.Fprintf(w, " %s=%s", k, v); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintln(w)
	return err
}
