// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the Municode parsing helpers.

package cli

import "testing"

func TestMcParseCity(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantAbbr string
		wantErr  bool
	}{
		{"Atlanta, GA", "Atlanta", "GA", false},
		{"  savannah , ga ", "savannah", "GA", false},
		{"St. Petersburg, FL", "St. Petersburg", "FL", false},
		{"Atlanta", "", "", true},
		{"Atlanta, Georgia", "", "", true},
		{"", "", "", true},
	}
	for _, c := range cases {
		name, abbr, err := mcParseCity(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("mcParseCity(%q) err=%v, wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && (name != c.wantName || abbr != c.wantAbbr) {
			t.Errorf("mcParseCity(%q) = (%q,%q), want (%q,%q)", c.in, name, abbr, c.wantName, c.wantAbbr)
		}
	}
}

func TestMcHTMLToText(t *testing.T) {
	in := `<div class="chunk-content"><p>Fences may not exceed <b>six feet</b> &amp; must be &#39;maintained&#39;.</p></div>`
	got := mcHTMLToText(in)
	want := "Fences may not exceed six feet & must be 'maintained'."
	if got != want {
		t.Errorf("mcHTMLToText = %q, want %q", got, want)
	}
}

func TestMcParseOrdHistory(t *testing.T) {
	text := "Some section text. (Code 1977, § 18-1008; Ord. No. 2006-45, § 1, 7-25-06; Ord. No. 2009-19(09-O-0798), § 2, 5-1-09)"
	refs := mcParseOrdHistory(text)
	if len(refs) < 2 {
		t.Fatalf("expected at least 2 ordinance refs, got %d (%+v)", len(refs), refs)
	}
	var found2006 bool
	for _, r := range refs {
		if r.Ordinance == "2006-45" {
			found2006 = true
			if r.Date != "7-25-06" {
				t.Errorf("ord 2006-45 date = %q, want 7-25-06", r.Date)
			}
		}
	}
	if !found2006 {
		t.Errorf("did not extract Ord. No. 2006-45 from %q (got %+v)", text, refs)
	}
}

func TestMcParseOrdHistoryBoulderFormat(t *testing.T) {
	// Boulder uses spelled-out "Ordinance No." in footnote prose, not the
	// abbreviated trailing-parenthetical form.
	text := "Footnotes: --- ( 1 ) --- Adopted by Ordinance No. 4705. Derived from Ordinance No. 3838, 1925 Code."
	refs := mcParseOrdHistory(text)
	got := map[string]bool{}
	for _, r := range refs {
		if r.Ordinance != "" {
			got["ord:"+r.Ordinance] = true
		}
		if r.CodeYear != "" {
			got["code:"+r.CodeYear] = true
		}
	}
	for _, want := range []string{"ord:4705", "ord:3838", "code:1925"} {
		if !got[want] {
			t.Errorf("Boulder format: missing %s; got refs %+v", want, refs)
		}
	}
}

func TestMcParseOrdHistoryNone(t *testing.T) {
	if refs := mcParseOrdHistory("A plain sentence with no annotations."); len(refs) != 0 {
		t.Errorf("expected no refs, got %+v", refs)
	}
}

func TestMcParseXrefs(t *testing.T) {
	text := "As provided in § 1-3 and Chapter 66, see also § 1-3 again and Article IV."
	xrefs := mcParseXrefs(text)
	if len(xrefs) == 0 {
		t.Fatalf("expected cross-references, got none")
	}
	// § 1-3 appears twice but must be deduped.
	count := 0
	for _, x := range xrefs {
		if x == "§ 1-3" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected § 1-3 deduped to 1 occurrence, got %d in %v", count, xrefs)
	}
}

func TestMcMentionsSection(t *testing.T) {
	cases := []struct {
		text    string
		section string
		want    bool
	}{
		{"See § 1-2 for details.", "1-2", true},
		{"As provided in Section 1-2.", "1-2", true},
		{"See § 1-20 for details.", "1-2", false}, // must not match longer section
		{"Enacted (Ord. 1-2-09).", "1-2", false},  // must not match a date-like token
		{"Refer to Sec. 16-28.001.", "16-28.001", true},
		{"No citation here.", "1-2", false},
	}
	for _, c := range cases {
		if got := mcMentionsSection(c.text, c.section); got != c.want {
			t.Errorf("mcMentionsSection(%q, %q) = %v, want %v", c.text, c.section, got, c.want)
		}
	}
}

func TestMcLibraryURL(t *testing.T) {
	got := mcLibraryURL("GA", "St. Petersburg", "")
	want := "https://library.municode.com/ga/st_petersburg/codes/code_of_ordinances"
	if got != want {
		t.Errorf("mcLibraryURL = %q, want %q", got, want)
	}
}
