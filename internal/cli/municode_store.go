// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Municode local-store helpers: the BFS code walk that populates
// the generic resources table with section documents, plus query helpers used
// by the transcendence commands. Not generated; preserved across regen.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"municode-pp-cli/internal/client"
	"municode-pp-cli/internal/store"
)

// mcStoredDoc is a synced section, stored as a generic "document" resource.
type mcStoredDoc struct {
	DocID      string     `json:"doc_id"`
	NodeID     string     `json:"node_id"`
	ClientID   int        `json:"client_id"`
	Client     string     `json:"client"`
	State      string     `json:"state"`
	ProductID  int        `json:"product_id"`
	JobID      int        `json:"job_id"`
	Title      string     `json:"title"`
	Text       string     `json:"text"`
	Citation   string     `json:"citation"`
	OrdHistory []mcOrdRef `json:"ord_history,omitempty"`
}

func mcStoreID(clientID int, docID string) string {
	return strconv.Itoa(clientID) + ":" + docID
}

// mcSyncCode walks the code's TOC tree and stores every section document.
// Content is delivered in chunks (a node fetch returns its whole chunk of
// Docs), so a `covered` set dedups and bounds the number of API calls. Returns
// the number of documents stored.
func mcSyncCode(ctx context.Context, c *client.Client, db *store.Store, res *mcResolved, maxNodes int, progress func(string)) (int, bool, error) {
	covered := map[string]bool{}
	stored := 0
	partial := false
	// BFS queue of node ids; start at the root (nodeId == productId).
	queue := []string{strconv.Itoa(res.ProductID)}
	visited := map[string]bool{}
	fetches := 0

	for len(queue) > 0 {
		if ctx.Err() != nil {
			return stored, true, nil
		}
		node := queue[0]
		queue = queue[1:]
		if visited[node] {
			continue
		}
		visited[node] = true

		// Enqueue children for the walk.
		children, err := mcTocChildren(ctx, c, res.ProductID, res.JobID, node)
		if err != nil {
			if ctx.Err() != nil {
				return stored, true, nil
			}
			return stored, partial, fmt.Errorf("walking TOC at %s: %w", node, err)
		}
		for _, ch := range children {
			if ch.Id != "" && !visited[ch.Id] {
				queue = append(queue, ch.Id)
			}
		}

		// Skip content fetch if this node's content is already covered.
		if covered[node] {
			continue
		}
		if maxNodes > 0 && fetches >= maxNodes {
			partial = true
			if progress != nil {
				progress(fmt.Sprintf("reached --max-nodes=%d; stopping content walk", maxNodes))
			}
			break
		}
		fetches++
		docs, err := mcContent(ctx, c, res.ProductID, res.JobID, node)
		if err != nil {
			if ctx.Err() != nil {
				return stored, true, nil
			}
			return stored, partial, fmt.Errorf("fetching content at %s: %w", node, err)
		}
		for _, d := range docs {
			covered[d.Id] = true
			text := mcHTMLToText(d.Content)
			title := mcHTMLToText(d.TitleHtml)
			if strings.TrimSpace(text) == "" && strings.TrimSpace(title) == "" {
				continue
			}
			rec := mcStoredDoc{
				DocID:      d.Id,
				NodeID:     d.Id,
				ClientID:   res.ClientID,
				Client:     res.ClientName,
				State:      res.StateAbbr,
				ProductID:  res.ProductID,
				JobID:      res.JobID,
				Title:      title,
				Text:       text,
				Citation:   mcLibraryURL(res.StateAbbr, res.ClientName, d.Id),
				OrdHistory: mcParseOrdHistory(text),
			}
			payload, err := json.Marshal(rec)
			if err != nil {
				return stored, partial, err
			}
			if err := db.Upsert(mcDocType, mcStoreID(res.ClientID, d.Id), payload); err != nil {
				return stored, partial, fmt.Errorf("storing %s: %w", d.Id, err)
			}
			stored++
		}
		if progress != nil && stored%200 == 0 && stored > 0 {
			progress(fmt.Sprintf("stored %d sections...", stored))
		}
	}
	return stored, partial, nil
}

// mcLoadCityDocs returns all stored documents for one municipality (by client id).
func mcLoadCityDocs(ctx context.Context, db *store.Store, clientID int) ([]mcStoredDoc, error) {
	return mcScanDocs(ctx, db, `SELECT data FROM resources WHERE resource_type = ? AND json_extract(data,'$.client_id') = ?`,
		mcDocType, clientID)
}

// mcScanDocs runs a SELECT data query and unmarshals each row into mcStoredDoc.
// Drain-first: the result set is fully scanned and closed before any caller
// issues follow-up queries.
func mcScanDocs(ctx context.Context, db *store.Store, query string, args ...any) ([]mcStoredDoc, error) {
	rows, err := db.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query documents: %w", err)
	}
	out := make([]mcStoredDoc, 0)
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scan document: %w", err)
		}
		if !data.Valid {
			continue
		}
		var d mcStoredDoc
		if err := json.Unmarshal([]byte(data.String), &d); err != nil {
			continue
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// mcSyncedCity is a distinct synced municipality derived from stored documents.
type mcSyncedCity struct {
	ClientID  int    `json:"client_id"`
	Client    string `json:"client"`
	State     string `json:"state"`
	ProductID int    `json:"product_id"`
	JobID     int    `json:"job_id"`
	Sections  int    `json:"sections"`
}

// mcSyncedCities lists the distinct municipalities present in the local store.
func mcSyncedCities(ctx context.Context, db *store.Store) ([]mcSyncedCity, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT json_extract(data,'$.client_id'),
		       json_extract(data,'$.client'),
		       json_extract(data,'$.state'),
		       json_extract(data,'$.product_id'),
		       MAX(json_extract(data,'$.job_id')),
		       COUNT(*)
		FROM resources WHERE resource_type = ?
		GROUP BY json_extract(data,'$.client_id')
		ORDER BY json_extract(data,'$.client')`, mcDocType)
	if err != nil {
		return nil, err
	}
	out := make([]mcSyncedCity, 0)
	for rows.Next() {
		var (
			cid, pid, jid, n sql.NullInt64
			client, state    sql.NullString
		)
		if err := rows.Scan(&cid, &client, &state, &pid, &jid, &n); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, mcSyncedCity{
			ClientID:  int(cid.Int64),
			Client:    client.String,
			State:     state.String,
			ProductID: int(pid.Int64),
			JobID:     int(jid.Int64),
			Sections:  int(n.Int64),
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// mcResolveCityArg resolves a "City, ST" argument to its synced client id by
// matching against the local store (no API call). Returns the matched city.
func mcSyncedCityByName(cities []mcSyncedCity, cityState string) (*mcSyncedCity, bool) {
	name, abbr, err := mcParseCity(cityState)
	if err != nil {
		return nil, false
	}
	for i := range cities {
		if strings.EqualFold(cities[i].Client, name) && strings.EqualFold(cities[i].State, abbr) {
			return &cities[i], true
		}
	}
	return nil, false
}
