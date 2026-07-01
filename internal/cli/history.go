// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Hand-authored. Ordinance-history lineage extraction from synced sections.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelHistoryCmd(flags *rootFlags) *cobra.Command {
	var flagSection string
	var byOrdinance string
	var dbPath string
	var limit int
	cmd := &cobra.Command{
		Use:   "history <city>",
		Short: "Extract a section's enacting/amending ordinance lineage from the local store",
		Long: "Turn the '(Ord. No. ..., date)' history annotations embedded in code sections into structured " +
			"ordinance / section / date rows.\n\n" +
			"Use --section to get the enacting ordinances for one section, or --by-ordinance to invert the " +
			"index and list every section a given ordinance enacted or amended. Requires the city to be cloned first.",
		Example:     "  municode-pp-cli history \"Atlanta, GA\" --section 1-2\n  municode-pp-cli history \"Atlanta, GA\" --by-ordinance 2006-45",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would extract ordinance history from the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"City, ST\" municipality argument is required"))
			}
			if err := mcRequireCityFormat(cmd, args[0]); err != nil {
				return err
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
			docs, err := mcLoadCityDocs(ctx, db, city.ClientID)
			if err != nil {
				return err
			}
			type row struct {
				Section    string     `json:"section"`
				NodeID     string     `json:"node_id"`
				Citation   string     `json:"citation"`
				OrdHistory []mcOrdRef `json:"ord_history"`
			}
			out := make([]row, 0)
			for i := range docs {
				d := docs[i]
				if len(d.OrdHistory) == 0 {
					continue
				}
				if flagSection != "" && !mcMatchSection(d, flagSection) {
					continue
				}
				if byOrdinance != "" {
					matched := make([]mcOrdRef, 0)
					for _, h := range d.OrdHistory {
						if strings.EqualFold(h.Ordinance, byOrdinance) {
							matched = append(matched, h)
						}
					}
					if len(matched) == 0 {
						continue
					}
					out = append(out, row{d.Title, d.NodeID, d.Citation, matched})
				} else {
					out = append(out, row{d.Title, d.NodeID, d.Citation, d.OrdHistory})
				}
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&flagSection, "section", "", "Limit to a section identifier (e.g. 1-2)")
	cmd.Flags().StringVar(&byOrdinance, "by-ordinance", "", "List every section enacted/amended by this ordinance number")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum rows to return (0 = unbounded)")
	return cmd
}
