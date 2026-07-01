// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for the local-store transcendence commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"municode-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// mcOpenLocal resolves the db path and opens the store, applying the
// missing-mirror guard. When the mirror does not exist it prints a sync hint,
// emits "[]" for machine output, and returns ok=false so the caller returns nil.
func mcOpenLocal(ctx context.Context, cmd *cobra.Command, flags *rootFlags, dbPath string) (db *store.Store, resolved string, ok bool, err error) {
	if dbPath == "" {
		dbPath = defaultDBPath("municode-pp-cli")
	}
	if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
		fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: municode-pp-cli clone \"<City, ST>\"\n", dbPath)
		if flags.asJSON || flags.agent {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil, dbPath, false, nil
	}
	s, oerr := store.OpenWithContext(ctx, dbPath)
	if oerr != nil {
		return nil, dbPath, false, fmt.Errorf("opening database: %w", oerr)
	}
	return s, dbPath, true, nil
}

// mcRequireCityFormat validates that a city argument is well-formed "City, ST".
// Malformed input is a usage error (exit 2); a well-formed but not-yet-cloned
// city is handled separately as a graceful empty result.
func mcRequireCityFormat(cmd *cobra.Command, cityState string) error {
	if _, _, err := mcParseCity(cityState); err != nil {
		_ = cmd.Usage()
		return usageErr(err)
	}
	return nil
}

// mcSearchDocs runs the store's FTS over document resources and returns the
// matching stored docs in relevance order.
func mcSearchDocs(db *store.Store, query string, limit int) ([]mcStoredDoc, error) {
	raws, err := db.Search(query, limit, mcDocType)
	if err != nil {
		return nil, err
	}
	out := make([]mcStoredDoc, 0, len(raws))
	for _, r := range raws {
		var d mcStoredDoc
		if json.Unmarshal(r, &d) == nil {
			out = append(out, d)
		}
	}
	return out, nil
}

// mcFirstSentence returns the first sentence of text that contains the term,
// or the first sentence overall when no match is found.
func mcFirstSentence(text, term string) string {
	sentences := regexp.MustCompile(`[^.!?]*[.!?]`).FindAllString(text, -1)
	lterm := strings.ToLower(term)
	for _, s := range sentences {
		if strings.Contains(strings.ToLower(s), lterm) {
			return strings.TrimSpace(s)
		}
	}
	if len(sentences) > 0 {
		return strings.TrimSpace(sentences[0])
	}
	if len(text) > 280 {
		return strings.TrimSpace(text[:280])
	}
	return strings.TrimSpace(text)
}

// mcMatchSection reports whether a stored doc corresponds to a section
// identifier like "16-28.001" (matched against title or node id).
func mcMatchSection(d mcStoredDoc, section string) bool {
	section = strings.TrimSpace(section)
	if section == "" {
		return false
	}
	lt := strings.ToLower(d.Title)
	ls := strings.ToLower(section)
	return strings.Contains(lt, ls) || strings.Contains(strings.ToLower(d.NodeID), strings.ReplaceAll(ls, "-", ""))
}
