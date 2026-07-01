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
	"time"

	"github.com/KnitSecurity/municode-pp-cli/internal/client"
	"github.com/KnitSecurity/municode-pp-cli/internal/store"
)

// mcStoredDoc is a synced section, stored as a generic "document" resource.
// The trailing fields are populated only for PDF-backed documents (Rules,
// ordinances) and stay empty/omitted for HTML code sections.
type mcStoredDoc struct {
	DocID      string     `json:"doc_id"`
	NodeID     string     `json:"node_id"`
	ParentID   string     `json:"parent_id,omitempty"`
	Depth      int        `json:"depth"`
	ClientID   int        `json:"client_id"`
	Client     string     `json:"client"`
	State      string     `json:"state"`
	ProductID  int        `json:"product_id"`
	JobID      int        `json:"job_id"`
	Title      string     `json:"title"`
	Text       string     `json:"text"`
	Citation   string     `json:"citation"`
	OrdHistory []mcOrdRef `json:"ord_history,omitempty"`
	// PDF-doc fields (Rules / ordinances):
	SourceURL     string `json:"source_url,omitempty"`     // original PDF download URL
	SourceFile    string `json:"source_file,omitempty"`    // OriginalFileName
	Extractor     string `json:"extractor,omitempty"`      // pdftotext | go | none
	TextExtracted *bool  `json:"text_extracted,omitempty"` // false when only a scan (no text)
	DocDate       string `json:"doc_date,omitempty"`       // SortDate
	Breadcrumb    string `json:"breadcrumb,omitempty"`     // "Rules > City Manager/Emergency"
}

// mcRuleDocType is the resource_type for Rules PDFs, distinct from HTML code
// sections ("document") so the two can be listed/read separately while sharing
// the same store and FTS index.
const mcRuleDocType = "munidoc"

func mcStoreID(clientID int, docID string) string {
	return strconv.Itoa(clientID) + ":" + docID
}

// mcChunkParent reconstructs a doc's TOC parent from a content chunk: the
// nearest earlier doc exactly one NodeDepth level shallower. Returns "" for a
// depth-0 doc or when no shallower ancestor precedes it in the chunk.
func mcChunkParent(docs []mcDoc, i int) string {
	if i < 0 || i >= len(docs) {
		return ""
	}
	d := docs[i]
	if d.NodeDepth <= 0 {
		return ""
	}
	for j := i - 1; j >= 0; j-- {
		if docs[j].NodeDepth == d.NodeDepth-1 {
			return docs[j].Id
		}
	}
	return ""
}

// mcSyncCode walks the code's TOC tree and stores every section document.
// Content is delivered in chunks (a node fetch returns its whole chunk of
// Docs), so a `covered` set dedups and bounds the number of API calls. Returns
// the number of documents stored.
func mcSyncCode(ctx context.Context, c *client.Client, db *store.Store, res *mcResolved, maxNodes int, progress func(string)) (int, bool, error) {
	covered := map[string]bool{}
	// storedIDs tracks distinct documents written so the returned count matches
	// the row count reported by `clones`. Overlapping chunk groups can upsert the
	// same section more than once; those re-writes must not inflate the total.
	storedIDs := map[string]bool{}
	stored := 0
	partial := false
	// BFS queue of node ids; start at the root (nodeId == productId).
	root := strconv.Itoa(res.ProductID)
	queue := []string{root}
	visited := map[string]bool{}
	// TOC hierarchy captured from the children walk: parentOf[child]=node.
	// depthOf tracks BFS depth (root=0). Used as a fallback when a doc's
	// within-chunk parent cannot be derived from the chunk's NodeDepth run.
	parentOf := map[string]string{}
	depthOf := map[string]int{root: 0}
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
			if ch.Id == "" {
				continue
			}
			if _, seen := parentOf[ch.Id]; !seen {
				parentOf[ch.Id] = node
				depthOf[ch.Id] = depthOf[node] + 1
			}
			if !visited[ch.Id] {
				queue = append(queue, ch.Id)
			}
		}

		// Skip the content fetch only when we already have this node's body
		// (see covered semantics below).
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
		for i, d := range docs {
			if d.Id == "" {
				continue
			}
			text := mcHTMLToText(d.Content)
			// Municode chunks a large subtree into a "chunk group": the requested
			// node's body is inlined, but its descendants come back as
			// content-less pointer docs (Content=null) whose real bodies live in
			// their own, deeper chunk-group fetch. A pointer must NOT be marked
			// covered — doing so is what suppressed the deeper fetch and left
			// sections as stubs — and we skip storing it, because this same node
			// is (or will be) fetched directly, which returns its body. Marking
			// covered only for content-bearing docs also stops the deeper fetch
			// from redundantly re-fetching a section already inlined in its group.
			if text == "" {
				continue
			}
			covered[d.Id] = true
			// Heading: prefer the HTML title, then the plain Title field (pointer
			// docs and some chunk docs carry only the latter).
			title := mcHTMLToText(d.TitleHtml)
			if title == "" {
				title = strings.TrimSpace(d.Title)
			}
			// Depth: prefer the API's NodeDepth; fall back to the BFS depth.
			depth := d.NodeDepth
			if depth == 0 {
				depth = depthOf[d.Id]
			}
			// Parent: nearest earlier doc in this chunk one level shallower
			// (reconstructs chapter->section within a chunk); otherwise the
			// TOC parent captured during the BFS children walk.
			parentID := mcChunkParent(docs, i)
			if parentID == "" {
				parentID = parentOf[d.Id]
			}
			rec := mcStoredDoc{
				DocID:      d.Id,
				NodeID:     d.Id,
				ParentID:   parentID,
				Depth:      depth,
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
			if !storedIDs[d.Id] {
				storedIDs[d.Id] = true
				stored++
			}
		}
		if progress != nil && stored%200 == 0 && stored > 0 {
			progress(fmt.Sprintf("stored %d sections...", stored))
		}
	}
	return stored, partial, nil
}

// mcSyncRules walks the MuniDocs "rules" tree, downloads each rule's PDF,
// extracts its text (pdftotext -> pure-Go -> scan-reference), and stores it as a
// munidoc resource. Returns (stored, partial, error). partial is true if the
// walk was cancelled (Ctrl-C / deadline). A single PDF that fails to download is
// skipped (reported via progress), not fatal — a re-clone retries it.
func mcSyncRules(ctx context.Context, c *client.Client, db *store.Store, res *mcResolved, mdProductID int, timeout time.Duration, progress func(string)) (int, bool, error) {
	stored := 0
	skipped := 0
	// DFS the folder tree from the rules root.
	stack := []string{mcRulesRootNode}
	visited := map[string]bool{}
	crumbOf := map[string]string{mcRulesRootNode: "Rules"}

	for len(stack) > 0 {
		if ctx.Err() != nil {
			return stored, true, nil
		}
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if visited[node] {
			continue
		}
		visited[node] = true

		children, err := mcMuniDocsChildren(ctx, c, mdProductID, node)
		if err != nil {
			if ctx.Err() != nil {
				return stored, true, nil
			}
			return stored, false, fmt.Errorf("walking rules at %s: %w", node, err)
		}
		for _, ch := range children {
			if ch.Id == "" {
				continue
			}
			if ch.Data.IsFolder {
				crumbOf[ch.Id] = crumbOf[node] + " > " + ch.Heading
				if !visited[ch.Id] {
					stack = append(stack, ch.Id)
				}
				continue
			}
			ok, err := mcStoreRuleDoc(ctx, c, db, res, mdProductID, ch, crumbOf[node], timeout)
			if err != nil {
				if ctx.Err() != nil {
					return stored, true, nil
				}
				return stored, false, err
			}
			if ok {
				stored++
			} else {
				skipped++
				if progress != nil {
					progress(fmt.Sprintf("skipped rule %q (PDF download failed; re-clone to retry)", ch.Heading))
				}
			}
			if progress != nil && stored%20 == 0 && stored > 0 {
				progress(fmt.Sprintf("stored %d rules...", stored))
			}
		}
	}
	if progress != nil && skipped > 0 {
		progress(fmt.Sprintf("rules: %d stored, %d skipped (download failures)", stored, skipped))
	}
	return stored, false, nil
}

// mcStoreRuleDoc fetches one rule leaf's metadata + PDF, extracts text, and
// stores it. Returns ok=false (nil error) when the PDF download fails so the
// caller can count it as skipped without aborting the clone.
func mcStoreRuleDoc(ctx context.Context, c *client.Client, db *store.Store, res *mcResolved, mdProductID int, node mcMuniDocNode, breadcrumb string, timeout time.Duration) (bool, error) {
	meta, err := mcMuniDocMetadata(ctx, c, mdProductID, node.Id)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, nil // metadata blip: skip, re-clone retries
	}
	url := mcMuniDocPDFURL(mdProductID, node.Id)
	pdfBytes, err := mcFetchPDF(ctx, url, timeout)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, nil // download failed: skip (non-fatal)
	}
	text, extractor := mcExtractPDF(ctx, pdfBytes)
	extracted := text != ""
	title := node.Heading
	if title == "" {
		title = meta.Heading
	}
	rec := mcStoredDoc{
		DocID:         node.Id,
		NodeID:        node.Id,
		ClientID:      res.ClientID,
		Client:        res.ClientName,
		State:         res.StateAbbr,
		ProductID:     mdProductID,
		Title:         title,
		Text:          text,
		Citation:      mcMuniDocLibraryURL(res, node.Id),
		SourceURL:     url,
		SourceFile:    meta.OriginalFileName,
		Extractor:     extractor,
		TextExtracted: &extracted,
		DocDate:       meta.SortDate,
		Breadcrumb:    breadcrumb,
	}
	payload, err := json.Marshal(rec)
	if err != nil {
		return false, err
	}
	if err := db.Upsert(mcRuleDocType, mcStoreID(res.ClientID, node.Id), payload); err != nil {
		return false, fmt.Errorf("storing rule %s: %w", node.Id, err)
	}
	return true, nil
}

// mcMuniDocLibraryURL builds the public library permalink for a MuniDocs node.
func mcMuniDocLibraryURL(res *mcResolved, nodeID string) string {
	return fmt.Sprintf("https://library.municode.com/%s/%s/munidocs/munidocs?nodeId=%s",
		strings.ToLower(res.StateAbbr), mcSlug(res.ClientName), nodeID)
}

// mcReadLocalSections returns the stored sections for a node from the clone:
// the node itself plus its direct children (approximating a live content
// chunk). Empty (not error) when the node is not cloned. Offline only.
func mcReadLocalSections(ctx context.Context, db *store.Store, clientID int, nodeID string) ([]mcStoredDoc, error) {
	return mcScanDocs(ctx, db, `SELECT data FROM resources
		WHERE resource_type = ? AND json_extract(data,'$.client_id') = ?
		  AND (json_extract(data,'$.node_id') = ? OR json_extract(data,'$.parent_id') = ?)
		ORDER BY json_extract(data,'$.depth'), json_extract(data,'$.node_id')`,
		mcDocType, clientID, nodeID, nodeID)
}

// mcLoadCityDocs returns all stored documents for one municipality (by client id).
func mcLoadCityDocs(ctx context.Context, db *store.Store, clientID int) ([]mcStoredDoc, error) {
	return mcScanDocs(ctx, db, `SELECT data FROM resources WHERE resource_type = ? AND json_extract(data,'$.client_id') = ?`,
		mcDocType, clientID)
}

// mcLoadCityRules returns all stored Rules (MuniDocs) for one municipality.
func mcLoadCityRules(ctx context.Context, db *store.Store, clientID int) ([]mcStoredDoc, error) {
	return mcScanDocs(ctx, db, `SELECT data FROM resources WHERE resource_type = ? AND json_extract(data,'$.client_id') = ?
		ORDER BY json_extract(data,'$.doc_date') DESC`,
		mcRuleDocType, clientID)
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
	ClientID   int    `json:"client_id"`
	Client     string `json:"client"`
	State      string `json:"state"`
	ProductID  int    `json:"product_id"`
	JobID      int    `json:"job_id"`
	Sections   int    `json:"sections"`
	LastSynced string `json:"last_synced,omitempty"`
}

// mcSyncedCities lists the distinct municipalities present in the local store.
func mcSyncedCities(ctx context.Context, db *store.Store) ([]mcSyncedCity, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT json_extract(data,'$.client_id'),
		       json_extract(data,'$.client'),
		       json_extract(data,'$.state'),
		       json_extract(data,'$.product_id'),
		       MAX(json_extract(data,'$.job_id')),
		       COUNT(*),
		       MAX(synced_at)
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
			lastSynced       sql.NullString
		)
		if err := rows.Scan(&cid, &client, &state, &pid, &jid, &n, &lastSynced); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out = append(out, mcSyncedCity{
			ClientID:   int(cid.Int64),
			Client:     client.String,
			State:      state.String,
			ProductID:  int(pid.Int64),
			JobID:      int(jid.Int64),
			Sections:   int(n.Int64),
			LastSynced: lastSynced.String,
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
