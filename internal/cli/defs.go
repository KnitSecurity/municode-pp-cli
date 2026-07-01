// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Hand-authored. Authoritative defined-term lookup from a code's Definitions sections.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelDefsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "defs <city> <term>",
		Short: "Return the authoritative definition of a term from a code's Definitions sections, with its citation",
		Long: "Look up the controlling definition of a term in a synced municipal code. Searches the local " +
			"store, prefers Definitions-family sections, and returns the defining clause plus its citation.\n\n" +
			"Use this to fetch the ONE controlling definition of a term. Do NOT use it for general full-text " +
			"hits across the code; use 'search' for that. Requires the city to be cloned first.",
		Example:     "  municode-pp-cli defs \"Atlanta, GA\" \"dwelling unit\"",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO;term=dwelling unit"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would look up a defined term in the local store")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both a \"City, ST\" municipality and a term are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, _, ok, err := mcOpenLocal(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			cities, err := mcSyncedCities(ctx, db)
			if err != nil {
				return err
			}
			city, found := mcSyncedCityByName(cities, args[0])
			if !found {
				fmt.Fprintf(cmd.ErrOrStderr(), "%q is not cloned yet\nrun: municode-pp-cli clone %q\n", args[0], args[0])
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			term := strings.Join(args[1:], " ")
			hits, err := mcSearchDocs(db, term, 80)
			if err != nil {
				return err
			}
			type defResult struct {
				Term       string `json:"term"`
				Definition string `json:"definition"`
				Section    string `json:"section"`
				NodeID     string `json:"node_id"`
				Citation   string `json:"citation"`
			}
			var best *mcStoredDoc
			bestScore := -1
			for i := range hits {
				if hits[i].ClientID != city.ClientID {
					continue
				}
				score := 0
				lt := strings.ToLower(hits[i].Title)
				if strings.Contains(lt, "defin") {
					score += 10
				}
				if strings.Contains(strings.ToLower(hits[i].Text), strings.ToLower(term)) {
					score += 1
				}
				if score > bestScore {
					bestScore = score
					best = &hits[i]
				}
			}
			out := make([]defResult, 0, 1)
			if best != nil {
				out = append(out, defResult{
					Term:       term,
					Definition: mcFirstSentence(best.Text, term),
					Section:    best.Title,
					NodeID:     best.NodeID,
					Citation:   best.Citation,
				})
			}
			if len(out) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no definition found for %q in %s; try 'search %q'\n", term, args[0], term)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	return cmd
}
