// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. Flag cloned codes whose upstream codification is newer.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelStaleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List cloned codes whose upstream codification is newer than your local copy",
		Long: "Check every cloned municipality against Municode's current latest codification job and report " +
			"which local mirrors are out of date. Use this to know which cities to re-clone before trusting " +
			"or publishing their text. Reads the local store and makes one live job-version check per city.",
		Example:     "  municode-pp-cli stale\n  municode-pp-cli stale --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would check cloned codes against upstream versions")
				return nil
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
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			type row struct {
				City      string `json:"city"`
				LocalJob  int    `json:"local_job"`
				LatestJob int    `json:"latest_job"`
				Stale     bool   `json:"stale"`
				Error     string `json:"error,omitempty"`
			}
			out := make([]row, 0, len(cities))
			for _, ci := range cities {
				r := row{City: ci.Client + ", " + ci.State, LocalJob: ci.JobID}
				latest, lerr := mcLatestJob(ctx, c, ci.ProductID)
				if lerr != nil {
					r.Error = lerr.Error()
				} else {
					r.LatestJob = latest
					r.Stale = latest > ci.JobID
				}
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	return cmd
}
