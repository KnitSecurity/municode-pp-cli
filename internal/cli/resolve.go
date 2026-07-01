// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. Resolve a "City, ST" name to its addressable Municode code.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResolveCmd(flags *rootFlags) *cobra.Command {
	var urlOnly bool
	cmd := &cobra.Command{
		Use:   "resolve <city>",
		Short: "Resolve a municipality name to its code (client, product, latest version) and library URL",
		Long: "Resolve a \"City, ST\" municipality name to the ids needed to read its code: " +
			"client id, the Code of Ordinances product id, and the latest codification version (job) id.\n\n" +
			"Use this to turn a human city name into an addressable handle before calling 'toc', 'read', or 'clone'.",
		Example:     "  municode-pp-cli resolve \"Atlanta, GA\"\n  municode-pp-cli resolve \"Savannah, GA\" --url",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve municipality")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"City, ST\" municipality argument is required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := mcResolve(ctx, c, args[0])
			if err != nil {
				return err
			}
			if urlOnly {
				fmt.Fprintln(cmd.OutOrStdout(), res.LibraryURL)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().BoolVar(&urlOnly, "url", false, "Print only the public library URL for the code")
	return cmd
}
