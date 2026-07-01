// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the clones inventory helper.

package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"municode-pp-cli/internal/store"
)

func clonesTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func putCloneDoc(t *testing.T, db *store.Store, clientID int, node string, doc map[string]any) {
	t.Helper()
	doc["client_id"] = clientID
	doc["node_id"] = node
	payload, _ := json.Marshal(doc)
	if err := db.Upsert(mcDocType, mcStoreID(clientID, node), json.RawMessage(payload)); err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestMcSyncedCitiesInventory(t *testing.T) {
	db := clonesTestStore(t)
	putCloneDoc(t, db, 1357, "A", map[string]any{"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931, "title": "a"})
	putCloneDoc(t, db, 1357, "B", map[string]any{"client": "Boulder", "state": "CO", "product_id": 18020, "job_id": 489931, "title": "b"})
	putCloneDoc(t, db, 1093, "C", map[string]any{"client": "Atlanta", "state": "GA", "product_id": 10376, "job_id": 487860, "title": "c"})

	cities, err := mcSyncedCities(context.Background(), db)
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	if len(cities) != 2 {
		t.Fatalf("want 2 cities, got %d", len(cities))
	}
	byID := map[int]mcSyncedCity{}
	for _, c := range cities {
		byID[c.ClientID] = c
	}
	if b := byID[1357]; b.Sections != 2 || b.ProductID != 18020 || b.JobID != 489931 || b.State != "CO" {
		t.Errorf("Boulder = %+v, want 2 sections/product 18020/job 489931/CO", b)
	}
	if a := byID[1093]; a.Sections != 1 || a.Client != "Atlanta" {
		t.Errorf("Atlanta = %+v, want 1 section", a)
	}
	// last_synced is populated by the store on upsert.
	if byID[1357].LastSynced == "" {
		t.Error("Boulder last_synced empty; expected a synced_at timestamp")
	}
}

// TestClonesInventoryFieldParity guards that the CLI `clones` output shape
// matches the MCP municode://clones resource shape key-for-key (plan F2 /
// U2-U3 parity). The MCP mcpCloneRow declares exactly these JSON keys.
func TestClonesInventoryFieldParity(t *testing.T) {
	b, _ := json.Marshal(clonesRow{City: "x", LastSynced: "t"})
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	want := []string{"city", "state", "client_id", "product_id", "job_id", "sections", "last_synced"}
	if len(m) != len(want) {
		t.Errorf("clonesRow has %d keys, want %d (%v vs %v)", len(m), len(want), keysOf(m), want)
	}
	for _, f := range want {
		if _, ok := m[f]; !ok {
			t.Errorf("clonesRow JSON missing %q (parity with municode://clones)", f)
		}
	}
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
