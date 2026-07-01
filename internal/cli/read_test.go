// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for offline read.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/KnitSecurity/municode-pp-cli/internal/store"
)

func readTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func putReadDoc(t *testing.T, db *store.Store, clientID int, node, parent, title, text string) {
	t.Helper()
	doc := mcStoredDoc{
		DocID: node, NodeID: node, ParentID: parent, ClientID: clientID,
		Client: "Boulder", State: "CO", Title: title, Text: text, Citation: "https://lib/" + node,
	}
	payload, _ := json.Marshal(doc)
	if err := db.Upsert(mcDocType, mcStoreID(clientID, node), payload); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestMcReadLocalSections(t *testing.T) {
	db := readTestStore(t)
	putReadDoc(t, db, 1357, "CH1", "18020", "Chapter 1", "chapter body")
	putReadDoc(t, db, 1357, "CH1_S1", "CH1", "Sec 1-1", "section one body")
	putReadDoc(t, db, 1357, "CH1_S2", "CH1", "Sec 1-2", "section two body")
	putReadDoc(t, db, 1357, "OTHER", "18020", "Other", "unrelated")
	ctx := context.Background()

	// Reading the chapter node returns the chapter plus its direct children.
	docs, err := mcReadLocalSections(ctx, db, 1357, "CH1")
	if err != nil {
		t.Fatalf("read local: %v", err)
	}
	got := map[string]string{}
	for _, d := range docs {
		got[d.NodeID] = d.Text
	}
	for _, n := range []string{"CH1", "CH1_S1", "CH1_S2"} {
		if _, ok := got[n]; !ok {
			t.Errorf("chapter read missing %s; got %v", n, keysOf(anyMap(got)))
		}
	}
	if _, ok := got["OTHER"]; ok {
		t.Error("chapter read leaked an unrelated section")
	}

	// Reading a leaf section returns just that section, with the stored text
	// AND citation faithfully carried through. This is the CLI half of the
	// KTD5/U4-scenario-5 parity guarantee: the offline `read` and the MCP
	// `resources/read` (internal/mcp mcpReadSection, covered by its own test)
	// both surface the same stored $.text/$.citation for a leaf node, so the
	// two offline surfaces cannot diverge on a section's content.
	leaf, err := mcReadLocalSections(ctx, db, 1357, "CH1_S1")
	if err != nil {
		t.Fatal(err)
	}
	if len(leaf) != 1 || leaf[0].Text != "section one body" {
		t.Errorf("leaf read = %d docs, want 1 with section one body", len(leaf))
	}
	if leaf[0].Citation != "https://lib/CH1_S1" {
		t.Errorf("leaf citation = %q, want the stored citation (parity with resources/read)", leaf[0].Citation)
	}

	// Absent node -> empty, no error (offline miss).
	miss, err := mcReadLocalSections(ctx, db, 1357, "NOPE")
	if err != nil {
		t.Fatal(err)
	}
	if len(miss) != 0 {
		t.Errorf("absent node returned %d docs, want 0", len(miss))
	}
}

func anyMap(m map[string]string) map[string]any {
	out := map[string]any{}
	for k, v := range m {
		out[k] = v
	}
	return out
}
