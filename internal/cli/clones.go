// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// Hand-authored. Inventory of locally cloned municipal codes.
//
// This command intentionally exposes no --db flag: it always reads the default
// clone store. That keeps the MCP-mirrored tool pinned to the default store so
// an MCP client cannot point it at an arbitrary filesystem path (plan KTD6/R7).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// clonesRow is the inventory output shape; it matches the MCP
// municode://clones resource field-for-field (plan U2/U3 parity).
type clonesRow struct {
	City       string `json:"city"`
	State      string `json:"state"`
	ClientID   int    `json:"client_id"`
	ProductID  int    `json:"product_id"`
	JobID      int    `json:"job_id"`
	Sections   int    `json:"sections"`
	LastSynced string `json:"last_synced,omitempty"`
}

func newClonesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clones",
		Short: "List locally cloned municipal codes (offline inventory)",
		Long: "List the municipalities cloned into the local store, with their codification version " +
			"(job id), section count, and last-synced time. Reads the local clone store only; makes no " +
			"API call. Use this to see what an agent can answer offline before deciding whether to " +
			"'clone' a new city or re-clone a stale one ('stale' checks freshness against upstream).",
		Example:     "  municode-pp-cli clones\n  municode-pp-cli clones --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list locally cloned codes")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, _, ok, err := mcOpenLocal(ctx, cmd, flags, "")
			if err != nil || !ok {
				return err
			}
			defer db.Close()

			cities, err := mcSyncedCities(ctx, db)
			if err != nil {
				return err
			}
			// Emit the same field shape as the municode://clones MCP resource
			// (city, not client) so the tool and resource surfaces match.
			out := make([]clonesRow, 0, len(cities))
			for _, c := range cities {
				out = append(out, clonesRow{
					City:       c.Client,
					State:      c.State,
					ClientID:   c.ClientID,
					ProductID:  c.ProductID,
					JobID:      c.JobID,
					Sections:   c.Sections,
					LastSynced: c.LastSynced,
				})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}
