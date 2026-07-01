// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Hand-authored. Cross-city topic comparison over the local store.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var flagCity []string
	var flagState string
	var dbPath string
	cmd := &cobra.Command{
		Use:   "compare <topic>",
		Short: "Lay one topic side by side across several synced cities, aligned by controlling section",
		Long: "Compare how multiple municipalities regulate one topic. For each requested city, finds the " +
			"top matching section in the local store and aligns them so the same regulation can be read side " +
			"by side.\n\nUse this to compare ONE topic across MULTIPLE cities. Do NOT use it for ranked hits " +
			"inside one city ('search') or to fetch a known section by id ('read'). Pass --city repeatedly, or " +
			"--state to include every synced city in a state. Cities must be cloned first.",
		Example:     "  municode-pp-cli compare \"short-term rental\" --city \"Atlanta, GA\" --city \"Savannah, GA\"\n  municode-pp-cli compare \"noise\" --state GA",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "topic=zoning;--state=CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compare a topic across synced cities")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a topic argument is required"))
			}
			if len(flagCity) == 0 && strings.TrimSpace(flagState) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide at least one --city or a --state"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, _, ok, err := mcOpenLocal(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			topic := args[0]
			cities, err := mcSyncedCities(ctx, db)
			if err != nil {
				return err
			}

			// Resolve the target set of synced cities.
			targets := make([]mcSyncedCity, 0)
			notSynced := make([]string, 0)
			if strings.TrimSpace(flagState) != "" {
				st := strings.ToUpper(strings.TrimSpace(flagState))
				for _, c := range cities {
					if strings.EqualFold(c.State, st) {
						targets = append(targets, c)
					}
				}
			}
			for _, name := range flagCity {
				if c, found := mcSyncedCityByName(cities, name); found {
					targets = append(targets, *c)
				} else {
					notSynced = append(notSynced, name)
				}
			}
			// Rank documents by relevance once, then pick top per city.
			hits, err := mcSearchDocs(db, topic, 500)
			if err != nil {
				return err
			}
			type sectionView struct {
				Citation string `json:"citation"`
				Heading  string `json:"heading"`
				Snippet  string `json:"snippet"`
			}
			type cityView struct {
				City    string       `json:"city"`
				Matched bool         `json:"matched"`
				Section *sectionView `json:"section,omitempty"`
			}
			views := make([]cityView, 0, len(targets))
			seen := map[int]bool{}
			for _, t := range targets {
				if seen[t.ClientID] {
					continue
				}
				seen[t.ClientID] = true
				cv := cityView{City: t.Client + ", " + t.State}
				for i := range hits {
					if hits[i].ClientID == t.ClientID {
						cv.Matched = true
						cv.Section = &sectionView{
							Citation: hits[i].Citation,
							Heading:  hits[i].Title,
							Snippet:  mcFirstSentence(hits[i].Text, topic),
						}
						break
					}
				}
				views = append(views, cv)
			}
			result := map[string]any{
				"topic":  topic,
				"cities": views,
			}
			if len(notSynced) > 0 {
				result["not_synced"] = notSynced
				result["note"] = "some requested cities are not cloned yet; run 'municode-pp-cli clone \"<City, ST>\"'"
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringArrayVar(&flagCity, "city", nil, "A \"City, ST\" to include (repeatable)")
	cmd.Flags().StringVar(&flagState, "state", "", "Include every synced city in this state")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	return cmd
}
