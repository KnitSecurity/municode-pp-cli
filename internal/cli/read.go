// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. Read a section's content as clean plaintext.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newReadCmd(flags *rootFlags) *cobra.Command {
	var citation bool
	cmd := &cobra.Command{
		Use:   "read <city> <node-id>",
		Short: "Read a code section's content as clean plaintext, with its citation",
		Long: "Fetch the content chunk for a TOC node and return each section's title and " +
			"HTML-stripped plaintext.\n\nUse 'toc' to find the node id. Use 'search' or 'defs' to " +
			"locate a section by topic instead of by id.",
		Example:     "  municode-pp-cli read \"Atlanta, GA\" PTIICOORENOR_CH1GEPR\n  municode-pp-cli read \"Atlanta, GA\" PTIICOORENOR_CH1GEPR --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO;node=TIT1GEAD"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read section content")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("both a \"City, ST\" municipality and a node id are required"))
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
			docs, err := mcContent(ctx, c, res.ProductID, res.JobID, args[1])
			if err != nil {
				return fmt.Errorf("reading %s: %w", args[1], err)
			}
			type section struct {
				NodeID     string     `json:"node_id"`
				Title      string     `json:"title"`
				Text       string     `json:"text"`
				Citation   string     `json:"citation"`
				OrdHistory []mcOrdRef `json:"ord_history,omitempty"`
			}
			out := make([]section, 0, len(docs))
			for _, d := range docs {
				text := mcHTMLToText(d.Content)
				title := mcHTMLToText(d.TitleHtml)
				if text == "" && title == "" {
					continue
				}
				out = append(out, section{
					NodeID:     d.Id,
					Title:      title,
					Text:       text,
					Citation:   mcLibraryURL(res.StateAbbr, res.ClientName, d.Id),
					OrdHistory: mcParseOrdHistory(text),
				})
			}
			if len(out) == 0 {
				return fmt.Errorf("no content found for node %q in %s", args[1], args[0])
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			var b strings.Builder
			for _, s := range out {
				if s.Title != "" {
					b.WriteString(s.Title)
					b.WriteString("\n")
				}
				if s.Text != "" {
					b.WriteString(s.Text)
					b.WriteString("\n")
				}
				if citation {
					b.WriteString(s.Citation)
					b.WriteString("\n")
				}
				b.WriteString("\n")
			}
			fmt.Fprint(cmd.OutOrStdout(), b.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&citation, "citation", false, "Include the library permalink citation in human output")
	return cmd
}
