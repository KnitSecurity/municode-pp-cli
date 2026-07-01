// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for PDF text extraction.

package cli

import (
	"context"
	"strings"
	"testing"
)

func TestMcCleanPDFText(t *testing.T) {
	cases := map[string]string{
		"  hello  \n\n\n\nworld  \n":     "hello\n\nworld",
		"line1   \nline2\r\nline3":       "line1\nline2\nline3",
		"\r\n\r\n  trimmed  \r\n\r\n":    "trimmed",
		"a\n\n\n\n\n\nb":                 "a\n\nb",
		"   spaces then tabs\t\t\n next": "spaces then tabs\n next",
	}
	for in, want := range cases {
		if got := mcCleanPDFText(in); got != want {
			t.Errorf("mcCleanPDFText(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestMcExtractPDFGracefulOnGarbage: non-PDF bytes must not panic and must report
// extractorNone (the scanned/undecodable signal), regardless of whether pdftotext
// is installed.
func TestMcExtractPDFGracefulOnGarbage(t *testing.T) {
	for _, data := range [][]byte{
		[]byte("this is not a pdf"),
		{},
		[]byte("%PDF-1.4\nbut truncated and broken"),
	} {
		text, extractor := mcExtractPDF(context.Background(), data)
		if text != "" {
			t.Errorf("garbage input produced text %q", text)
		}
		if extractor != extractorNone {
			t.Errorf("garbage input extractor = %q, want %q", extractor, extractorNone)
		}
	}
}

// TestMcGoPDFTextNoPanic guards the recover in the pure-Go path.
func TestMcGoPDFTextNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("mcGoPDFText panicked: %v", r)
		}
	}()
	if got := mcGoPDFText([]byte(strings.Repeat("\x00\xff", 500))); got != "" {
		t.Errorf("expected empty text from binary garbage, got %q", got)
	}
}
