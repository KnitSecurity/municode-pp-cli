// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. Browse a code's table-of-contents tree.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type mcTocOut struct {
	NodeID   string `json:"node_id"`
	Heading  string `json:"heading"`
	Depth    int    `json:"depth"`
	Citation string `json:"citation,omitempty"`
}

func newTocCmd(flags *rootFlags) *cobra.Command {
	var node string
	var depth int
	cmd := &cobra.Command{
		Use:   "toc <city>",
		Short: "Browse a municipal code's table of contents (chapters, articles, sections)",
		Long: "Browse the table of contents for a city's code. With no --node it lists the top-level " +
			"parts/chapters; pass --node to expand a specific node, and --depth to recurse.\n\n" +
			"Use this to discover the node id you want before calling 'read'.",
		Example:     "  municode-pp-cli toc \"Atlanta, GA\"\n  municode-pp-cli toc \"Atlanta, GA\" --node PTIICOORENOR_CH1GEPR --depth 2",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch table of contents")
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
			root := node
			if root == "" {
				root = strconv.Itoa(res.ProductID)
			}
			if depth < 1 {
				depth = 1
			}
			out := make([]mcTocOut, 0)
			var walk func(nodeID string, d int) error
			walk = func(nodeID string, d int) error {
				kids, err := mcTocChildren(ctx, c, res.ProductID, res.JobID, nodeID)
				if err != nil {
					return err
				}
				for _, k := range kids {
					out = append(out, mcTocOut{
						NodeID:   k.Id,
						Heading:  strings.TrimSpace(k.Heading),
						Depth:    d,
						Citation: mcLibraryURL(res.StateAbbr, res.ClientName, k.Id),
					})
					if d < depth {
						if err := walk(k.Id, d+1); err != nil {
							return err
						}
					}
				}
				return nil
			}
			if err := walk(root, 1); err != nil {
				return fmt.Errorf("walking TOC: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&node, "node", "", "TOC node id to expand (default: code root)")
	cmd.Flags().IntVar(&depth, "depth", 1, "Levels of the tree to expand")
	return cmd
}
