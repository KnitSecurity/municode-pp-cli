// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the clone MCP resource surface.

package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/KnitSecurity/municode-pp-cli/internal/store"
)

// seedDefaultStore points mcpDBPath() at a fresh temp home and returns a store
// opened at that default path, so tests can exercise the store-pinned refresh
// path exactly as the running server would.
func seedDefaultStore(t *testing.T) *store.Store {
	t.Helper()
	t.Setenv("MUNICODE_HOME", t.TempDir())
	path, err := mcpDBPath()
	if err != nil {
		t.Fatalf("mcpDBPath: %v", err)
	}
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("open default store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// TestBuildCloneResourcesReflectsStore proves the resource list is rebuilt from
// current store contents on every call (no cached snapshot): a section added
// after the first build shows up on the next build — the mechanism that makes
// an in-session clone visible without a restart (U5, R8).
func TestBuildCloneResourcesReflectsStore(t *testing.T) {
	db := seedDefaultStore(t)

	// Empty store: just the inventory resource, no sections.
	if got := len(buildCloneResources()); got != 1 {
		t.Fatalf("empty store: got %d resources, want 1 (inventory only)", got)
	}

	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1", "parent_id": "18020", "depth": 1,
		"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931,
		"title": "Chapter 1", "text": "chapter body", "citation": "https://lib/CH1",
	})
	first := buildCloneResources()
	if len(first) != 2 {
		t.Fatalf("after 1 clone section: got %d resources, want 2 (inventory + CH1)", len(first))
	}

	// Simulate an in-session clone adding another section.
	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1_S1", "parent_id": "CH1", "depth": 2,
		"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931,
		"title": "Sec 1-1", "text": "section body", "citation": "https://lib/CH1_S1",
	})
	second := buildCloneResources()
	if len(second) != 3 {
		t.Fatalf("after in-session clone: got %d resources, want 3 (inventory + CH1 + CH1_S1)", len(second))
	}
	uris := map[string]bool{}
	for _, r := range second {
		uris[r.Resource.URI] = true
	}
	if !uris[sectionURI(1357, "CH1_S1")] {
		t.Errorf("newly cloned section %s not present after rebuild", sectionURI(1357, "CH1_S1"))
	}
}

// TestBuildCloneResourcesIdempotent guards that repeated rebuilds on an
// unchanged store yield the same URI set with no duplicates.
func TestBuildCloneResourcesIdempotent(t *testing.T) {
	db := seedDefaultStore(t)
	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1", "depth": 1, "client": "Boulder", "state": "CO",
		"product_id": 18020, "job_id": 489931, "title": "Chapter 1", "text": "b", "citation": "c",
	})
	countURIs := func() map[string]int {
		m := map[string]int{}
		for _, r := range buildCloneResources() {
			m[r.Resource.URI]++
		}
		return m
	}
	a, b := countURIs(), countURIs()
	if len(a) != len(b) {
		t.Fatalf("rebuild not idempotent: %d vs %d URIs", len(a), len(b))
	}
	for uri, n := range b {
		if n != 1 {
			t.Errorf("URI %s appears %d times; want exactly 1 (no duplicates)", uri, n)
		}
		if a[uri] != n {
			t.Errorf("URI %s count drifted between rebuilds: %d vs %d", uri, a[uri], n)
		}
	}
}

// TestSectionReadIndependentOfList is the regression for plan U5 scenario 2:
// a section resolves via the template read handler even when the listable set
// was never (re)built — reads must not depend on the list snapshot.
func TestSectionReadIndependentOfList(t *testing.T) {
	db := seedDefaultStore(t)
	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1", "depth": 1, "client": "Boulder", "state": "CO",
		"product_id": 18020, "job_id": 489931,
		"title": "Chapter 1", "text": "chapter body", "citation": "https://lib/CH1",
	})
	// Deliberately do NOT call buildCloneResources/RefreshCloneResources first.
	req := mcplib.ReadResourceRequest{}
	req.Params.URI = sectionURI(1357, "CH1")
	contents, err := handleSectionResource(context.Background(), req)
	if err != nil {
		t.Fatalf("section read without a list refresh failed: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("got %d contents, want 1", len(contents))
	}
	tc, ok := contents[0].(mcplib.TextResourceContents)
	if !ok || tc.Text == "" {
		t.Fatalf("expected non-empty text content, got %#v", contents[0])
	}
}

// TestIsCloneTool checks the refresh trigger predicate.
func TestIsCloneTool(t *testing.T) {
	if !isCloneTool("clone") {
		t.Error("clone should trigger a refresh")
	}
	for _, name := range []string{"clones", "read", "search", "context", ""} {
		if isCloneTool(name) {
			t.Errorf("%q should not trigger a refresh", name)
		}
	}
}

// TestOnAfterCallToolRefreshWiring checks that only a clone tool call reaches
// the server-refresh path, and that a nil request/server is handled safely.
func TestOnAfterCallToolRefreshWiring(t *testing.T) {
	seedDefaultStore(t)
	s := server.NewMCPServer("test", "0", server.WithResourceCapabilities(false, true))
	RegisterResources(s)

	calls := 0
	getServer := func() *server.MCPServer { calls++; return s }

	newReq := func(name string) *mcplib.CallToolRequest {
		r := &mcplib.CallToolRequest{}
		r.Params.Name = name
		return r
	}

	onAfterCallTool(newReq("clone"), getServer)
	if calls != 1 {
		t.Errorf("clone call: getServer invoked %d times, want 1", calls)
	}
	onAfterCallTool(newReq("search"), getServer)
	if calls != 1 {
		t.Errorf("non-clone call reached refresh path (getServer calls now %d)", calls)
	}
	// nil request and nil server must not panic.
	onAfterCallTool(nil, getServer)
	onAfterCallTool(newReq("clone"), func() *server.MCPServer { return nil })
}

func mcpTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func mcpPutDoc(t *testing.T, db *store.Store, clientID int, doc map[string]any) {
	t.Helper()
	doc["client_id"] = clientID
	payload, _ := json.Marshal(doc)
	id := doc["node_id"].(string)
	if err := db.Upsert(mcpDocType, "c:"+id, json.RawMessage(payload)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestMcpListSectionsAndRead(t *testing.T) {
	db := mcpTestStore(t)
	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1", "parent_id": "18020", "depth": 1,
		"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931,
		"title": "Chapter 1", "text": "chapter body", "citation": "https://lib/CH1",
	})
	mcpPutDoc(t, db, 1357, map[string]any{
		"node_id": "CH1_S1", "parent_id": "CH1", "depth": 2,
		"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931,
		"title": "Sec 1-1", "text": "section body", "citation": "https://lib/CH1_S1",
	})
	ctx := context.Background()

	secs, err := mcpListSections(ctx, db)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(secs) != 2 {
		t.Fatalf("want 2 sections, got %d", len(secs))
	}
	byNode := map[string]mcpSectionRow{}
	for _, s := range secs {
		byNode[s.NodeID] = s
	}
	if s := byNode["CH1_S1"]; s.ParentID != "CH1" || s.Depth != 2 || s.Title != "Sec 1-1" {
		t.Errorf("CH1_S1 = %+v, want parent CH1 depth 2 title 'Sec 1-1'", s)
	}

	title, text, cite, found, err := mcpReadSection(ctx, db, 1357, "CH1_S1")
	if err != nil || !found {
		t.Fatalf("read CH1_S1: found=%v err=%v", found, err)
	}
	if title != "Sec 1-1" || text != "section body" || cite != "https://lib/CH1_S1" {
		t.Errorf("read = (%q,%q,%q)", title, text, cite)
	}

	// Not found for an absent node.
	if _, _, _, found, _ := mcpReadSection(ctx, db, 1357, "NOPE"); found {
		t.Error("expected not-found for absent node")
	}
}

func TestMcpReadSectionInjectionSafe(t *testing.T) {
	db := mcpTestStore(t)
	mcpPutDoc(t, db, 1, map[string]any{"node_id": "S1", "title": "t", "text": "x", "citation": "c"})
	// A malicious node id must be bound as a parameter, not executed. Expect a
	// clean not-found, and the table must still be intact afterward.
	_, _, _, found, err := mcpReadSection(context.Background(), db, 1, "'; DROP TABLE resources;--")
	if err != nil {
		t.Fatalf("injection input errored: %v", err)
	}
	if found {
		t.Error("injection input unexpectedly matched")
	}
	if _, _, _, ok, _ := mcpReadSection(context.Background(), db, 1, "S1"); !ok {
		t.Error("table damaged after injection attempt — S1 no longer readable")
	}
}

func TestMcpEmptyStoreListsNothing(t *testing.T) {
	db := mcpTestStore(t) // no documents
	secs, err := mcpListSections(context.Background(), db)
	if err != nil {
		t.Fatalf("list empty: %v", err)
	}
	if len(secs) != 0 {
		t.Errorf("want 0 sections on empty store, got %d", len(secs))
	}
}

func TestParseSectionURI(t *testing.T) {
	cases := []struct {
		uri      string
		wantCID  int
		wantNode string
		wantOK   bool
	}{
		{"municode://clone/1357/TIT1GEAD_CH1COIN_1-1-1LEIN", 1357, "TIT1GEAD_CH1COIN_1-1-1LEIN", true},
		{"municode://clone/1/S1", 1, "S1", true},
		{"municode://clones", 0, "", false},
		{"municode://clone/notanumber/S1", 0, "", false},
		{"municode://clone/1357/", 0, "", false},
		{"http://evil/1/2", 0, "", false},
	}
	for _, c := range cases {
		cid, node, ok := parseSectionURI(c.uri)
		if ok != c.wantOK || (ok && (cid != c.wantCID || node != c.wantNode)) {
			t.Errorf("parseSectionURI(%q) = (%d,%q,%v), want (%d,%q,%v)", c.uri, cid, node, ok, c.wantCID, c.wantNode, c.wantOK)
		}
	}
	// Round-trip.
	if got := sectionURI(1357, "A_B-1"); got != "municode://clone/1357/A_B-1" {
		t.Errorf("sectionURI round-trip = %q", got)
	}
}
