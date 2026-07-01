// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Regression test for clone completeness: Municode delivers a large subtree as a
// chunk-group TOC of content-less "pointer" docs (Content=null, only a plain
// Title), whose real bodies live in deeper chunk-group fetches. The clone walk
// must fetch those deeper chunks rather than treat a pointer as fully covered.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"municode-pp-cli/internal/client"
	"municode-pp-cli/internal/config"
	"municode-pp-cli/internal/store"
)

// mcFakeDoc mirrors the fields of a Municode CodesContent doc the clone reads.
type mcFakeDoc struct {
	Id        string  `json:"Id"`
	NodeDepth int     `json:"NodeDepth"`
	Title     string  `json:"Title"`
	TitleHtml *string `json:"TitleHtml"`
	Content   *string `json:"Content"`
}

func strptr(s string) *string { return &s }

// mcMuniStub is a minimal Municode API: a TOC tree plus per-node content chunks.
// Large containers return their descendants as content-less pointer docs, exactly
// like the real API's [SHOW_TOC] chunk groups.
func mcMuniStub(t *testing.T) *httptest.Server {
	t.Helper()
	children := map[string][]string{
		"900":    {"TITLE"},      // root -> one title
		"TITLE":  {"CH1", "CH2"}, // title -> two chapters (large: pointers only)
		"CH1":    {"CH1_S1"},     // chapter 1 -> one section
		"CH2":    {"CH2_S1"},     // chapter 2 -> one section
		"CH1_S1": {},
		"CH2_S1": {},
	}
	// content[node] = the Docs the CodesContent endpoint returns for that node.
	pointer := func(id string, depth int, title string) mcFakeDoc {
		return mcFakeDoc{Id: id, NodeDepth: depth, Title: title} // Content/TitleHtml nil
	}
	body := func(id string, depth int, heading, text string) mcFakeDoc {
		return mcFakeDoc{Id: id, NodeDepth: depth, Title: heading,
			TitleHtml: strptr("<h>" + heading + "</h>"), Content: strptr("<p>" + text + "</p>")}
	}
	content := map[string][]mcFakeDoc{
		"900": {{Id: "900", NodeDepth: 0}}, // root: empty content doc
		// Fetching the title returns its own heading PLUS content-less pointers
		// for everything beneath it — the bug trigger.
		"TITLE": {
			body("TITLE", 1, "The Title", "title intro"),
			pointer("CH1", 2, "Chapter 1"),
			pointer("CH1_S1", 3, "Section 1-1"),
			pointer("CH2", 2, "Chapter 2"),
			pointer("CH2_S1", 3, "Section 2-1"),
		},
		// Deeper fetches inline the real bodies.
		"CH1":    {body("CH1", 2, "Chapter 1", "chapter one body"), body("CH1_S1", 3, "Section 1-1", "section one one body")},
		"CH2":    {body("CH2", 2, "Chapter 2", "chapter two body"), body("CH2_S1", 3, "Section 2-1", "section two one body")},
		"CH1_S1": {body("CH1_S1", 3, "Section 1-1", "section one one body")},
		"CH2_S1": {body("CH2_S1", 3, "Section 2-1", "section two one body")},
	}
	toNodes := func(ids []string) []map[string]any {
		out := make([]map[string]any, 0, len(ids))
		for _, id := range ids {
			out = append(out, map[string]any{"Id": id, "Heading": id})
		}
		return out
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		node := r.URL.Query().Get("nodeId")
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/codesToc/children":
			_ = json.NewEncoder(w).Encode(toNodes(children[node]))
		case "/CodesContent":
			_ = json.NewEncoder(w).Encode(map[string]any{"Docs": content[node]})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestCloneFetchesDeepChunkGroupContent(t *testing.T) {
	srv := mcMuniStub(t)
	defer srv.Close()

	c := client.New(&config.Config{BaseURL: srv.URL}, 0, 0)
	c.NoCache = true

	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	res := &mcResolved{ClientID: 42, ClientName: "Testville", StateAbbr: "TS", ProductID: 900, JobID: 1}
	n, _, err := mcSyncCode(context.Background(), c, db, res, 0, nil)
	if err != nil {
		t.Fatalf("mcSyncCode: %v", err)
	}

	docs, err := mcLoadCityDocs(context.Background(), db, 42)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	byID := map[string]mcStoredDoc{}
	for _, d := range docs {
		byID[d.NodeID] = d
	}

	// Every leaf section must be stored WITH its real body — not left as a stub
	// because a parent chunk listed it as a content-less pointer.
	want := map[string]string{
		"CH1_S1": "section one one body",
		"CH2_S1": "section two one body",
		"CH1":    "chapter one body",
		"CH2":    "chapter two body",
		"TITLE":  "title intro",
	}
	for id, wantText := range want {
		d, ok := byID[id]
		if !ok {
			t.Errorf("node %s missing from clone (stored=%d): %v", id, n, keysOfStored(byID))
			continue
		}
		if d.Text != wantText {
			t.Errorf("node %s text = %q, want %q (chunk-group body not fetched)", id, d.Text, wantText)
		}
	}
}

func keysOfStored(m map[string]mcStoredDoc) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
