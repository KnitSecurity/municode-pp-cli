// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
// Hand-authored. Read a section's content as clean plaintext, offline from the
// local clone when present (--data-source local|auto) or live (--data-source
// live). The offline path uses the default clone store only and makes no API
// call — see the plan's KTD3/R6.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"municode-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

// mcReadOut is one section in `read` output; shared by the offline and live paths.
type mcReadOut struct {
	NodeID     string     `json:"node_id"`
	Title      string     `json:"title"`
	Text       string     `json:"text"`
	Citation   string     `json:"citation"`
	OrdHistory []mcOrdRef `json:"ord_history,omitempty"`
}

func newReadCmd(flags *rootFlags) *cobra.Command {
	var citation bool
	cmd := &cobra.Command{
		Use:   "read <city> <node-id>",
		Short: "Read a code section's content as clean plaintext, with its citation",
		Long: "Return each section's title and HTML-stripped plaintext for a TOC node.\n\n" +
			"Data source (--data-source): 'auto' (default) reads the local clone when the section is " +
			"present and falls back to a live API call otherwise; 'local' reads only the clone and makes " +
			"no network call (empty if the city is not cloned); 'live' always fetches from the API.\n\n" +
			"Grouping note: the offline read (local/auto) returns the section plus its direct child " +
			"sections from the clone; a live read returns the API's content chunk, which for a container " +
			"node (a title or chapter) may span more levels. For a leaf section both return the same text " +
			"and citation.\n\n" +
			"Use 'toc' to find the node id. Use 'search' or 'defs' to locate a section by topic. The " +
			"offline path reads the default clone store only.",
		Example:     "  municode-pp-cli read \"Atlanta, GA\" PTIICOORENOR_CH1GEPR\n  municode-pp-cli read \"Boulder, CO\" TIT1GEAD --data-source local",
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

			ds := flags.dataSource
			if ds == "" {
				ds = "auto"
			}

			// Offline path: local or auto, read from the default clone store.
			if ds == "local" || ds == "auto" {
				out, served, err := readLocalSections(ctx, args[0], args[1])
				switch {
				case err != nil && ds == "local":
					return err
				case err != nil:
					// auto: a readable-but-degraded local store (e.g. locked
					// mid-clone) must not fail the read — fall back to the live
					// API per KTD3's auto contract instead of surfacing the error.
					fmt.Fprintf(cmd.ErrOrStderr(), "local clone unavailable (%v); falling back to live\n", err)
				case served:
					return renderRead(cmd, flags, citation, out)
				case ds == "local":
					fmt.Fprintf(cmd.ErrOrStderr(), "no local clone of %s (or node %q not cloned)\nrun: municode-pp-cli clone %q\n", args[0], args[1], args[0])
					if flags.asJSON || flags.agent {
						fmt.Fprintln(cmd.OutOrStdout(), "[]")
					}
					return nil
				}
				// auto with a miss or a degraded store: fall through to live.
			}

			// Live path.
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
			out := make([]mcReadOut, 0, len(docs))
			for _, d := range docs {
				text := mcHTMLToText(d.Content)
				title := mcHTMLToText(d.TitleHtml)
				if text == "" && title == "" {
					continue
				}
				out = append(out, mcReadOut{
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
			return renderRead(cmd, flags, citation, out)
		},
	}
	cmd.Flags().BoolVar(&citation, "citation", false, "Include the library permalink citation in human output")
	return cmd
}

// readLocalSections reads a node's sections from the default clone store.
// served is false (with nil error) when there is no clone or the node is not
// present, so an auto caller can fall back to live. Never calls the network.
func readLocalSections(ctx context.Context, cityState, nodeID string) (out []mcReadOut, served bool, err error) {
	dbPath := defaultDBPath("municode-pp-cli")
	if _, statErr := os.Stat(dbPath); statErr != nil {
		return nil, false, nil
	}
	db, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, false, nil // treat an unreadable store as "not served" so auto falls back
	}
	defer db.Close()

	cities, err := mcSyncedCities(ctx, db)
	if err != nil {
		return nil, false, err
	}
	city, found := mcSyncedCityByName(cities, cityState)
	if !found {
		return nil, false, nil
	}
	docs, err := mcReadLocalSections(ctx, db, city.ClientID, nodeID)
	if err != nil {
		return nil, false, err
	}
	if len(docs) == 0 {
		return nil, false, nil
	}
	out = make([]mcReadOut, 0, len(docs))
	for _, d := range docs {
		out = append(out, mcReadOut{
			NodeID:     d.NodeID,
			Title:      d.Title,
			Text:       d.Text,
			Citation:   d.Citation,
			OrdHistory: d.OrdHistory,
		})
	}
	return out, true, nil
}

func renderRead(cmd *cobra.Command, flags *rootFlags, citation bool, out []mcReadOut) error {
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
}
