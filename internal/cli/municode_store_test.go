// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the Municode store hierarchy helpers.

package cli

import (
	"encoding/json"
	"testing"
)

func TestMcChunkParent(t *testing.T) {
	// A chapter chunk: chapter (depth 2), then its sections (depth 3), with a
	// nested article (depth 3) that has its own sub-section (depth 4).
	docs := []mcDoc{
		{Id: "CH1", NodeDepth: 2},          // 0: chapter (chunk root)
		{Id: "CH1_S1", NodeDepth: 3},       // 1: section -> CH1
		{Id: "CH1_S2", NodeDepth: 3},       // 2: section -> CH1
		{Id: "CH1_ART1", NodeDepth: 3},     // 3: article -> CH1
		{Id: "CH1_ART1_SS1", NodeDepth: 4}, // 4: sub-section -> CH1_ART1
	}
	cases := []struct {
		i          int
		wantParent string
	}{
		{0, ""},         // chunk root: no in-chunk parent
		{1, "CH1"},      // section under chapter
		{2, "CH1"},      // section under chapter
		{3, "CH1"},      // article under chapter
		{4, "CH1_ART1"}, // sub-section under the article, not the chapter
	}
	for _, c := range cases {
		if got := mcChunkParent(docs, c.i); got != c.wantParent {
			t.Errorf("mcChunkParent(docs, %d) = %q, want %q", c.i, got, c.wantParent)
		}
	}
	// Out-of-range and depth-0 are safe.
	if got := mcChunkParent(docs, -1); got != "" {
		t.Errorf("mcChunkParent out-of-range = %q, want \"\"", got)
	}
	if got := mcChunkParent([]mcDoc{{Id: "ROOT", NodeDepth: 0}}, 0); got != "" {
		t.Errorf("mcChunkParent depth-0 = %q, want \"\"", got)
	}
}

func TestMcStoredDocBackwardCompatibleDecode(t *testing.T) {
	// A record written before parent_id/depth existed must still decode.
	old := `{"doc_id":"S1","node_id":"S1","client_id":1357,"client":"Boulder","state":"CO","title":"Sec 1","text":"body","citation":"u"}`
	var d mcStoredDoc
	if err := json.Unmarshal([]byte(old), &d); err != nil {
		t.Fatalf("decode legacy record: %v", err)
	}
	if d.ParentID != "" || d.Depth != 0 {
		t.Errorf("legacy record: ParentID=%q Depth=%d, want empty/0", d.ParentID, d.Depth)
	}
	if d.NodeID != "S1" || d.Client != "Boulder" {
		t.Errorf("legacy record lost fields: %+v", d)
	}
}

func TestMcStoredDocRoundTripHierarchy(t *testing.T) {
	in := mcStoredDoc{NodeID: "CH1_S1", ParentID: "CH1", Depth: 3, Title: "Sec 1"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out mcStoredDoc
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.ParentID != "CH1" || out.Depth != 3 {
		t.Errorf("round-trip: ParentID=%q Depth=%d, want CH1/3", out.ParentID, out.Depth)
	}
}
