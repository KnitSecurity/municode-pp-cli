// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored. PDF text extraction with capability detection: prefer the
// pdftotext binary (poppler) when present for better layout on multi-column
// legal PDFs, fall back to a pure-Go extractor so the CLI stays self-contained,
// and report when neither yields text (a scanned/image PDF the caller records as
// a reference).

package cli

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// Extractor names recorded on each stored doc so a later re-scan (e.g. after
// installing poppler or adding OCR) knows how the text was produced.
const (
	extractorPdftotext = "pdftotext"
	extractorGo        = "go"
	extractorNone      = "none"
)

// mcPdftotextAvailable reports whether the optional pdftotext binary is on PATH.
func mcPdftotextAvailable() bool {
	_, err := exec.LookPath("pdftotext")
	return err == nil
}

// mcExtractorNote returns a short parenthetical describing which extractor will
// be used, so the clone notice tells the user whether they'd benefit from
// installing poppler.
func mcExtractorNote() string {
	if mcPdftotextAvailable() {
		return " (using pdftotext for best layout)"
	}
	return " (using the built-in Go extractor; install pdftotext/poppler for better layout — see the README)"
}

// mcExtractPDF extracts cleaned plaintext from PDF bytes. It tries pdftotext
// first (better structure), then the pure-Go extractor, and returns
// ("", extractorNone) when neither produces text — the signal that the PDF is
// almost certainly a scan and needs OCR. Never panics on malformed input.
func mcExtractPDF(ctx context.Context, data []byte) (text, extractor string) {
	if mcPdftotextAvailable() {
		if t := mcCleanPDFText(mcPdftotext(ctx, data)); t != "" {
			return t, extractorPdftotext
		}
	}
	if t := mcCleanPDFText(mcGoPDFText(data)); t != "" {
		return t, extractorGo
	}
	return "", extractorNone
}

// mcPdftotext runs `pdftotext -layout` reading the PDF from stdin and writing
// text to stdout. Returns "" on any error.
func mcPdftotext(ctx context.Context, data []byte) string {
	cmd := exec.CommandContext(ctx, "pdftotext", "-layout", "-nopgbrk", "-", "-")
	cmd.Stdin = bytes.NewReader(data)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

// mcGoPDFText extracts text with the pure-Go reader. ledongthuc/pdf can panic on
// some malformed PDFs, so the recover keeps a bad document from aborting a clone.
func mcGoPDFText(data []byte) (text string) {
	defer func() {
		if recover() != nil {
			text = ""
		}
	}()
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	b, err := r.GetPlainText()
	if err != nil {
		return ""
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, b); err != nil {
		return ""
	}
	return buf.String()
}

var (
	mcPDFTrailingWS = regexp.MustCompile(`[ \t]+\n`)
	mcPDFBlankRuns  = regexp.MustCompile(`\n{3,}`)
)

// mcCleanPDFText normalizes extracted text without destroying the line/paragraph
// structure (which -layout preserves for tables): strips trailing whitespace per
// line and collapses runs of 3+ blank lines to a single blank line.
func mcCleanPDFText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = mcPDFTrailingWS.ReplaceAllString(s, "\n")
	s = mcPDFBlankRuns.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
