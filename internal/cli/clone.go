// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored flagship: full offline AI-referenceable clone of a code.

package cli

import (
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
	var flagNoExport bool
	var dbPath string
	var maxNodes int

	cmd := &cobra.Command{
		Use:   "clone <city>",
		Short: "Pull a municipality's entire code into a local store and an AI-ready Markdown tree in one command",
		Long: "Clone the full current code of a municipality for offline / AI reference. Walks the entire " +
			"table of contents, fetches every section, stores it in local SQLite with full-text search, and " +
			"builds the ordinance-history lineage index. It also writes a clean Markdown/text tree an agent " +
			"can read directly with no further API calls: by default into a per-city folder next to the " +
			"database in the data directory, named for the city (e.g. atlanta-ga). Run 'doctor' to see the " +
			"resolved data directory. Use --export DIR to choose the location, or --no-export to store only " +
			"the database.\n\n" +
			"Scope: current authoritative text plus the full ordinance-change lineage embedded in each " +
			"section. Verbatim text of superseded code versions is a paid Municode (CodeBank) feature and is " +
			"not included. After clone, 'search', 'read', 'defs', 'history', 'xref', 'compare', and 'diff' all " +
			"work offline against the local store.\n\n" +
			"Timing: a full code can be thousands of sections, delivered in small chunk groups, so a large " +
			"code can take several minutes. The clone runs to completion in a single pass — start it and " +
			"walk away; there is no need to run it twice. Interrupting it (Ctrl-C) leaves a resumable " +
			"partial: re-running skips already-stored sections.",
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
			// A full clone runs to completion in a single pass — no artificial
			// wall-clock cap — so even a large code can be cloned set-and-forget.
			// Each HTTP request still honors --timeout; the overall walk is bounded
			// only by the code's size (and Ctrl-C, which leaves a resumable
			// partial). Users would rather wait once than run the command twice.
			ctx := cmd.Context()

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
			// Set expectations up front: a full code can be thousands of sections,
			// and Municode serves them in small chunk groups, so a big code takes a
			// while. It runs to completion in one pass — no need to re-run. Printed
			// to stderr so it never mixes into JSON/agent output on stdout.
			if !flags.quiet {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"Cloning the full code for %s, %s. Larger codes have thousands of sections and can take several minutes — this runs to completion in a single pass, so you can start it and walk away.\n",
					res.ClientName, res.StateAbbr)
			}
			count, partial, err := mcSyncCode(ctx, c, db, res, maxNodes, progress)
			if err != nil {
				return fmt.Errorf("cloning %s: %w", args[0], err)
			}
			// Timestamp the clone so snapshots can be kept on disk and compared
			// across time (pair with the job_id, which is the codification version).
			clonedAt := time.Now().UTC().Format(time.RFC3339)

			// Export a readable Markdown/text tree. By default it goes to a
			// per-city subfolder next to the database (in the resolved data
			// directory), named for the city, so a bare `clone` produces a
			// browsable copy with no extra flags. --export overrides the location;
			// --no-export skips writing files (store only). Automated verify/
			// dogfood runs skip the default export to avoid surprise file writes.
			exportDir := flagExport
			if exportDir == "" && !flagNoExport && !cliutil.IsDogfoodEnv() && !cliutil.IsVerifyEnv() {
				exportDir = filepath.Join(filepath.Dir(dbPath), mcCitySlug(res.ClientName, res.StateAbbr))
			}
			exported := 0
			if exportDir != "" {
				docs, derr := mcLoadCityDocs(ctx, db, res.ClientID)
				if derr != nil {
					return derr
				}
				// A self-describing manifest (city, version, timestamp, counts)
				// so a future clone can be compared against this snapshot.
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
				n, eerr := mcExportMarkdown(exportDir, docs, manifest)
				if eerr != nil {
					return eerr
				}
				exported = n
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
			if exportDir != "" {
				result["export_dir"] = exportDir
				result["exported_files"] = exported
			}
			if partial {
				result["partial"] = true
				if maxNodes > 0 {
					result["max_nodes"] = maxNodes
					result["note"] = "partial clone: --max-nodes cap was applied; drop it (or raise it) to clone the full code"
				} else {
					result["note"] = "partial clone: interrupted before completion; re-run to continue (already-stored sections are skipped)"
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagExport, "export", "", "Export the Markdown/text tree to this directory (default: a per-city folder next to the database)")
	cmd.Flags().BoolVar(&flagNoExport, "no-export", false, "Do not write the Markdown/text tree; store only the database")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	cmd.Flags().IntVar(&maxNodes, "max-nodes", 0, "Cap the number of content chunks fetched (0 = whole code)")
	return cmd
}

// mcExportMarkdown writes one Markdown file per stored section into dir, plus a
// clone-manifest.json describing the snapshot. Returns the number of section
// files written. Shared by the clone command's default and --export paths.
func mcExportMarkdown(dir string, docs []mcStoredDoc, manifest map[string]any) (int, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("creating export dir: %w", err)
	}
	if mdata, merr := json.MarshalIndent(manifest, "", "  "); merr == nil {
		if werr := os.WriteFile(filepath.Join(dir, "clone-manifest.json"), mdata, 0o644); werr != nil {
			return 0, fmt.Errorf("writing clone manifest: %w", werr)
		}
	}
	written := 0
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
		if err := os.WriteFile(filepath.Join(dir, name), []byte(b.String()), 0o644); err != nil {
			return written, fmt.Errorf("writing %s: %w", name, err)
		}
		written++
	}
	return written, nil
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
