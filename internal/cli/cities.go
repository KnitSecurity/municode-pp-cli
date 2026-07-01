// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. List municipalities in a state, optionally filtered by name.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCitiesCmd(flags *rootFlags) *cobra.Command {
	var state string
	var query string
	cmd := &cobra.Command{
		Use:         "cities --state <ST> [--query <text>]",
		Short:       "List municipalities in a state that have a Municode-hosted code",
		Example:     "  municode-pp-cli cities --state GA\n  municode-pp-cli cities --state FL --query beach",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--state=CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 && len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list municipalities")
				return nil
			}
			if strings.TrimSpace(state) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--state is required (two-letter abbreviation, e.g. GA)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			raw, err := c.Get(ctx, "/Clients/stateAbbr", map[string]string{"stateAbbr": strings.ToUpper(state)})
			if err != nil {
				return fmt.Errorf("listing municipalities in %s: %w", strings.ToUpper(state), err)
			}
			var clients []mcClient
			_ = json.Unmarshal(raw, &clients)
			type row struct {
				ClientID int    `json:"client_id"`
				Name     string `json:"name"`
				State    string `json:"state"`
				Website  string `json:"website,omitempty"`
			}
			out := make([]row, 0, len(clients))
			for _, cl := range clients {
				if query != "" && !strings.Contains(strings.ToLower(cl.ClientName), strings.ToLower(query)) {
					continue
				}
				out = append(out, row{cl.ClientID, cl.ClientName, strings.ToUpper(state), cl.Website})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&state, "state", "", "Two-letter state abbreviation (e.g. GA)")
	cmd.Flags().StringVar(&query, "query", "", "Filter municipalities by name substring")
	return cmd
}
