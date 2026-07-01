// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Hand-authored. Intra-code cross-reference graph from synced sections.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelXrefCmd(flags *rootFlags) *cobra.Command {
	var flagSection string
	var flagInbound bool
	var dbPath string
	cmd := &cobra.Command{
		Use:   "xref <city>",
		Short: "List what a section cites (outbound) and what cites it (inbound) in a synced code",
		Long: "Build the cross-reference graph for a section of a synced code. By default lists the outbound " +
			"references the section makes ('see § X', 'Chapter Y'); with --inbound lists the sections that " +
			"reference this one.\n\nRequires the city to be cloned first.",
		Example:     "  municode-pp-cli xref \"Atlanta, GA\" --section 1-2\n  municode-pp-cli xref \"Atlanta, GA\" --section 1-2 --inbound",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO;--section=1-2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would build a cross-reference graph from the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"City, ST\" municipality argument is required"))
			}
			if strings.TrimSpace(flagSection) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--section is required"))
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
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			docs, err := mcLoadCityDocs(ctx, db, city.ClientID)
			if err != nil {
				return err
			}
			var target *mcStoredDoc
			for i := range docs {
				if mcMatchSection(docs[i], flagSection) {
					target = &docs[i]
					break
				}
			}
			if target == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "section %q not found in %s\n", flagSection, args[0])
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "{}")
				}
				return nil
			}
			result := map[string]any{
				"section":  target.Title,
				"node_id":  target.NodeID,
				"citation": target.Citation,
			}
			if flagInbound {
				type ref struct {
					Section  string `json:"section"`
					NodeID   string `json:"node_id"`
					Citation string `json:"citation"`
				}
				inbound := make([]ref, 0)
				for i := range docs {
					if docs[i].NodeID == target.NodeID {
						continue
					}
					if mcMentionsSection(docs[i].Text, flagSection) {
						inbound = append(inbound, ref{docs[i].Title, docs[i].NodeID, docs[i].Citation})
					}
				}
				result["inbound"] = inbound
				result["inbound_count"] = len(inbound)
			} else {
				outbound := mcParseXrefs(target.Text)
				result["outbound"] = outbound
				result["outbound_count"] = len(outbound)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagSection, "section", "", "Section identifier to analyze (e.g. 1-2)")
	cmd.Flags().BoolVar(&flagInbound, "inbound", false, "List sections that reference this one instead of its outbound references")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	return cmd
}
