// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Hand-authored. Drift detection: local clone vs current upstream code.

package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"municode-pp-cli/internal/client"
	"municode-pp-cli/internal/cliutil"

	"github.com/spf13/cobra"
)

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var maxNodes int
	cmd := &cobra.Command{
		Use:   "diff <city>",
		Short: "Show which sections changed in the live code since you cloned it (added, removed, reworded)",
		Long: "Compare your local clone of a municipality's code against the CURRENT upstream code and report " +
			"the sections that were added, removed, or reworded since you cloned.\n\n" +
			"This is drift detection against the live code, not a diff between two historical supplements " +
			"(Municode's version archive is a paid feature). Pair it with 'stale' (which flags codes at the " +
			"version level) for section-level change detail. Requires the city to be cloned first.",
		Example:     "  municode-pp-cli diff \"Atlanta, GA\"\n  municode-pp-cli diff \"Atlanta, GA\" --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "city=Boulder, CO"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff the local clone against the live code")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a \"City, ST\" municipality argument is required"))
			}
			if err := mcRequireCityFormat(cmd, args[0]); err != nil {
				return err
			}
			budget := 15 * time.Minute
			if flags.timeout > budget {
				budget = flags.timeout
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), budget)
			defer cancel()
			if cliutil.IsDogfoodEnv() && (maxNodes == 0 || maxNodes > 3) {
				maxNodes = 3
			}
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
			storedDocs, err := mcLoadCityDocs(ctx, db, city.ClientID)
			if err != nil {
				return err
			}
			stored := make(map[string]mcStoredDoc, len(storedDocs))
			for _, d := range storedDocs {
				stored[d.NodeID] = d
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := mcResolve(ctx, c, args[0])
			if err != nil {
				return err
			}
			current, err := mcCollectCode(ctx, c, res, maxNodes)
			if err != nil {
				return fmt.Errorf("fetching current code: %w", err)
			}
			// A timed-out walk returns a partial `current` map; the removed-section
			// pass would then flag every not-yet-fetched section as "removed".
			walkPartial := ctx.Err() != nil

			type change struct {
				NodeID   string `json:"node_id"`
				Title    string `json:"title"`
				Citation string `json:"citation"`
			}
			added := make([]change, 0)
			removed := make([]change, 0)
			changed := make([]change, 0)
			for node, cur := range current {
				prev, ok := stored[node]
				if !ok {
					added = append(added, change{node, cur.Title, cur.Citation})
				} else if strings.TrimSpace(prev.Text) != strings.TrimSpace(cur.Text) {
					changed = append(changed, change{node, cur.Title, cur.Citation})
				}
			}
			// Removed-section detection is only valid when the walk covered the
			// whole code: a capped or timed-out walk leaves unfetched sections
			// that would otherwise be falsely reported as removed.
			if maxNodes == 0 && !walkPartial {
				for node, prev := range stored {
					if _, ok := current[node]; !ok {
						removed = append(removed, change{node, prev.Title, prev.Citation})
					}
				}
			}

			result := map[string]any{
				"city":          res.ClientName + ", " + res.StateAbbr,
				"local_job":     city.JobID,
				"current_job":   res.JobID,
				"version_moved": res.JobID != city.JobID,
				"added":         added,
				"removed":       removed,
				"changed":       changed,
				"summary":       fmt.Sprintf("%d added, %d removed, %d reworded", len(added), len(removed), len(changed)),
			}
			if maxNodes > 0 || walkPartial {
				result["partial"] = true
				if maxNodes > 0 {
					result["max_nodes"] = maxNodes
				}
				result["note"] = "partial diff: the walk did not cover the whole code (timeout or --max-nodes cap), so removed-section detection was skipped; re-run with a higher --timeout or --max-nodes"
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path (default: resolved data directory)")
	cmd.Flags().IntVar(&maxNodes, "max-nodes", 0, "Cap the number of content chunks fetched from upstream (0 = whole code)")
	return cmd
}

// mcCollectCode walks the live code and returns documents keyed by node id
// WITHOUT writing to the store (used for drift comparison).
func mcCollectCode(ctx context.Context, c *client.Client, res *mcResolved, maxNodes int) (map[string]mcStoredDoc, error) {
	out := map[string]mcStoredDoc{}
	covered := map[string]bool{}
	visited := map[string]bool{}
	queue := []string{strconv.Itoa(res.ProductID)}
	fetches := 0
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return out, nil
		}
		node := queue[0]
		queue = queue[1:]
		if visited[node] {
			continue
		}
		visited[node] = true
		children, err := mcTocChildren(ctx, c, res.ProductID, res.JobID, node)
		if err != nil {
			if ctx.Err() != nil {
				return out, nil
			}
			return out, err
		}
		for _, ch := range children {
			if ch.Id != "" && !visited[ch.Id] {
				queue = append(queue, ch.Id)
			}
		}
		if covered[node] {
			continue
		}
		if maxNodes > 0 && fetches >= maxNodes {
			break
		}
		fetches++
		docs, err := mcContent(ctx, c, res.ProductID, res.JobID, node)
		if err != nil {
			if ctx.Err() != nil {
				return out, nil
			}
			return out, err
		}
		for _, d := range docs {
			covered[d.Id] = true
			text := mcHTMLToText(d.Content)
			title := mcHTMLToText(d.TitleHtml)
			if strings.TrimSpace(text) == "" && strings.TrimSpace(title) == "" {
				continue
			}
			out[d.Id] = mcStoredDoc{
				DocID:    d.Id,
				NodeID:   d.Id,
				Title:    title,
				Text:     text,
				Citation: mcLibraryURL(res.StateAbbr, res.ClientName, d.Id),
			}
		}
	}
	return out, nil
}
