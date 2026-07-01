// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the clone MCP resource surface.

package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"municode-pp-cli/internal/store"
)

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
