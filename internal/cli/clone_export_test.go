// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the clone Markdown export helpers.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMcCitySlug(t *testing.T) {
	cases := map[[2]string]string{
		{"Boulder", "CO"}:    "boulder-co",
		{"St. Louis", "MO"}:  "st-louis-mo",
		{"O'Fallon", "IL"}:   "o-fallon-il",
		{"  Acworth ", "ga"}: "acworth-ga",
		{"", ""}:             "code",
	}
	for in, want := range cases {
		if got := mcCitySlug(in[0], in[1]); got != want {
			t.Errorf("mcCitySlug(%q,%q) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestMcExportMarkdown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "boulder-co")
	docs := []mcStoredDoc{
		{DocID: "TIT1", Title: "Title 1", Text: "title body", Citation: "https://lib/TIT1"},
		{DocID: "TIT1_S1", Title: "Section 1-1", Text: "section body", Citation: "https://lib/TIT1_S1",
			OrdHistory: []mcOrdRef{{Raw: "Ord. No. 4705"}}},
	}
	manifest := map[string]any{"city": "Boulder, CO", "sections": len(docs)}

	n, err := mcExportMarkdown(dir, docs, manifest)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if n != 2 {
		t.Fatalf("wrote %d files, want 2", n)
	}

	// Manifest exists and round-trips.
	mdata, err := os.ReadFile(filepath.Join(dir, "clone-manifest.json"))
	if err != nil {
		t.Fatalf("manifest: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(mdata, &m); err != nil || m["city"] != "Boulder, CO" {
		t.Errorf("manifest = %s (err %v)", mdata, err)
	}

	// A section file carries heading, body, history, and source.
	got, err := os.ReadFile(filepath.Join(dir, "TIT1_S1.md"))
	if err != nil {
		t.Fatalf("read section file: %v", err)
	}
	want := "# Section 1-1\n\nsection body\n\n## History\n\n- Ord. No. 4705\n\nSource: https://lib/TIT1_S1\n"
	if string(got) != want {
		t.Errorf("section markdown =\n%q\nwant\n%q", got, want)
	}
}

func boolPtr(b bool) *bool { return &b }

func TestMcExportRulesMarkdown(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rules")
	docs := []mcStoredDoc{
		{DocID: "R1", Title: "2014 Prohibiting Smoking on Municipal Campus", Text: "No smoking is permitted...",
			SourceURL: "https://func/munidocDownload/31060/R1/pdf", Citation: "https://lib/R1",
			Breadcrumb: "Rules > City Manager/Emergency", DocDate: "2014-01-01T00:00:00",
			Extractor: extractorPdftotext, TextExtracted: boolPtr(true)},
		{DocID: "R2", Title: "Scanned Old Rule", Text: "",
			SourceURL: "https://func/munidocDownload/31060/R2/pdf",
			Extractor: extractorNone, TextExtracted: boolPtr(false)},
	}
	n, err := mcExportPDFDocsMarkdown(dir, docs)
	if err != nil {
		t.Fatalf("export rules: %v", err)
	}
	if n != 2 {
		t.Fatalf("wrote %d files, want 2", n)
	}
	// Text-extracted rule carries its body + metadata + source.
	got, _ := os.ReadFile(filepath.Join(dir, "R1.md"))
	for _, want := range []string{"# 2014 Prohibiting Smoking", "> Rules > City Manager/Emergency · 2014", "No smoking is permitted", "Source PDF: https://func"} {
		if !bytes.Contains(got, []byte(want)) {
			t.Errorf("R1.md missing %q; got:\n%s", want, got)
		}
	}
	// Scanned rule shows the placeholder, not empty body.
	scan, _ := os.ReadFile(filepath.Join(dir, "R2.md"))
	if !bytes.Contains(scan, []byte("Scanned document")) {
		t.Errorf("R2.md should note it's scanned; got:\n%s", scan)
	}
}
