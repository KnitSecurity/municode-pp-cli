// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored flagship: full offline AI-referenceable clone of a code.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"municode-pp-cli/internal/cliutil"
	"municode-pp-cli/internal/store"

	"github.com/spf13/cobra"
)

func newNovelCloneCmd(flags *rootFlags) *cobra.Command {
	var flagExport string
	var dbPath string
	var maxNodes int

	cmd := &cobra.Command{
		Use:   "clone <city>",
		Short: "Pull a municipality's entire code into a local store and an AI-ready Markdown tree in one command",
		Long: "Clone the full current code of a municipality for offline / AI reference. Walks the entire " +
			"table of contents, fetches every section, stores it in local SQLite with full-text search, and " +
			"builds the ordinance-history lineage index. With --export it also writes a clean Markdown/text " +
			"tree an agent can read directly with no further API calls.\n\n" +
			"Scope: current authoritative text plus the full ordinance-change lineage embedded in each " +
			"section. Verbatim text of superseded code versions is a paid Municode (CodeBank) feature and is " +
			"not included. After clone, 'search', 'read', 'defs', 'history', 'xref', 'compare', and 'diff' all " +
			"work offline against the local store.",
		Example:     "  municode-pp-cli clone \"Atlanta, GA\"\n  municode-pp-cli clone \"Atlanta, GA\" --export ./atlanta-code",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would clone the full code into the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"City, ST\" municipality argument is required"))
			}
			// Cloning a full code legitimately takes minutes; use a generous
			// budget (honoring a larger user --timeout) rather than the short
			// per-request default. The walk returns partial results on deadline.
			budget := 15 * time.Minute
			if flags.timeout > budget {
				budget = flags.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), budget)
			defer cancel()

			// Curtail the live walk under the dogfood matrix's flat timeout.
			if cliutil.IsDogfoodEnv() && (maxNodes == 0 || maxNodes > 3) {
				maxNodes = 3
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := mcResolve(ctx, c, args[0])
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("municode-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			human := wantsHumanTable(cmd.OutOrStdout(), flags)
			progress := func(msg string) {
				if human {
					fmt.Fprintln(cmd.ErrOrStderr(), "clone:", msg)
				}
			}
			count, partial, err := mcSyncCode(ctx, c, db, res, maxNodes, progress)
			if err != nil {
				return fmt.Errorf("cloning %s: %w", args[0], err)
			}
			// Timestamp the clone so snapshots can be kept on disk and compared
			// across time (pair with the job_id, which is the codification version).
			clonedAt := time.Now().UTC().Format(time.RFC3339)

			exported := 0
			if flagExport != "" {
				docs, derr := mcLoadCityDocs(ctx, db, res.ClientID)
				if derr != nil {
					return derr
				}
				if err := os.MkdirAll(flagExport, 0o755); err != nil {
					return fmt.Errorf("creating export dir: %w", err)
				}
				// Write a self-describing manifest (city, version, timestamp,
				// counts) so a future clone can be compared against this snapshot.
				manifest := map[string]any{
					"city":        res.ClientName + ", " + res.StateAbbr,
					"client_id":   res.ClientID,
					"product_id":  res.ProductID,
					"job_id":      res.JobID,
					"cloned_at":   clonedAt,
					"sections":    count,
					"partial":     partial,
					"library_url": res.LibraryURL,
				}
				if mdata, merr := json.MarshalIndent(manifest, "", "  "); merr == nil {
					if werr := os.WriteFile(filepath.Join(flagExport, "clone-manifest.json"), mdata, 0o644); werr != nil {
						return fmt.Errorf("writing clone manifest: %w", werr)
					}
				}
				for _, d := range docs {
					name := mcSafeFile(d.DocID) + ".md"
					var b strings.Builder
					if d.Title != "" {
						b.WriteString("# " + d.Title + "\n\n")
					}
					b.WriteString(d.Text + "\n\n")
					if len(d.OrdHistory) > 0 {
						b.WriteString("## History\n\n")
						for _, h := range d.OrdHistory {
							b.WriteString("- " + h.Raw + "\n")
						}
						b.WriteString("\n")
					}
					b.WriteString("Source: " + d.Citation + "\n")
					if err := os.WriteFile(filepath.Join(flagExport, name), []byte(b.String()), 0o644); err != nil {
						return fmt.Errorf("writing %s: %w", name, err)
					}
					exported++
				}
			}

			result := map[string]any{
				"city":        res.ClientName + ", " + res.StateAbbr,
				"client_id":   res.ClientID,
				"product_id":  res.ProductID,
				"job_id":      res.JobID,
				"cloned_at":   clonedAt,
				"sections":    count,
				"db":          dbPath,
				"library_url": res.LibraryURL,
			}
			if flagExport != "" {
				result["export_dir"] = flagExport
				result["exported_files"] = exported
			}
			if partial {
				result["partial"] = true
				if maxNodes > 0 {
					result["max_nodes"] = maxNodes
					result["note"] = "partial clone: --max-nodes cap was applied; raise it to clone the full code"
				} else {
					result["note"] = "partial clone: the walk timed out; re-run to continue (already-stored sections are skipped) or raise --timeout"
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagExport, "export", "", "Also export a clean Markdown/text tree to this directory")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	cmd.Flags().IntVar(&maxNodes, "max-nodes", 0, "Cap the number of content chunks fetched (0 = whole code)")
	return cmd
}

func mcSafeFile(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	out := r.Replace(s)
	if out == "" {
		return "section"
	}
	if len(out) > 120 {
		out = out[:120]
	}
	return out
}
