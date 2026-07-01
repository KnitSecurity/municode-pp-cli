// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the clone Markdown export helpers.

package cli

import (
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
