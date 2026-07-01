// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored MCP resources exposing the local municipal-code clone as a
// navigable, readable corpus. Not generated; keep regen-mergeable (see
// AGENTS.md). Handlers query the default clone store directly in package mcp
// (mirroring handleSearch/handleSQL); they never call the network and never
// honor a client-supplied db path (the MCP surface is pinned to the default
// store — see the plan's KTD6).

package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"municode-pp-cli/internal/store"
)

// mcpDocType is the resource_type under which clone sections are stored.
const mcpDocType = "document"

// clonesURI is the static inventory resource; sectionURITemplate addresses a
// single cloned section.
const (
	clonesURI          = "municode://clones"
	sectionURITemplate = "municode://clone/{clientId}/{nodeId}"
	sectionURIPrefix   = "municode://clone/"
)

// RegisterResources wires the clone resource surface onto the server: the
// section read template (once) and the listable resources (inventory + one per
// cloned section). Safe to call with no clone present — the list is simply
// empty. Called from cmd/municode-pp-mcp/main.go after RegisterTools.
func RegisterResources(s *server.MCPServer) {
	tmpl := mcplib.NewResourceTemplate(
		sectionURITemplate,
		"Municipal code section",
		mcplib.WithTemplateDescription("A single cloned code section by client id and node id. Text is offline from the local clone; no network call."),
		mcplib.WithTemplateMIMEType("text/plain"),
	)
	s.AddResourceTemplate(tmpl, handleSectionResource)
	RefreshCloneResources(s)
}

// RefreshCloneResources rebuilds the listable resource set from the current
// store contents and replaces the server's resource list. Re-callable so an
// in-session clone becomes visible without a restart (see U5). The read
// template registered in RegisterResources is unaffected.
func RefreshCloneResources(s *server.MCPServer) {
	resources := buildCloneResources()
	s.SetResources(resources...)
}

// RefreshHooks returns server hooks that re-list the clone resources after a
// clone tool call completes, so a city cloned mid-session appears in
// resources/list without a server restart (U5, R8). getServer defers the
// server reference because hooks are installed at construction time, before the
// *server.MCPServer exists (main.go assigns it, then the closure reads it).
func RefreshHooks(getServer func() *server.MCPServer) *server.Hooks {
	h := &server.Hooks{}
	h.AddAfterCallTool(func(_ context.Context, _ any, req *mcplib.CallToolRequest, _ any) {
		onAfterCallTool(req, getServer)
	})
	return h
}

// onAfterCallTool refreshes the clone resource list when — and only when — a
// clone tool call just completed. Tolerant of a nil request or a not-yet-set
// server so it is safe to fire during startup or from tests.
func onAfterCallTool(req *mcplib.CallToolRequest, getServer func() *server.MCPServer) {
	if req == nil || !isCloneTool(req.Params.Name) {
		return
	}
	if s := getServer(); s != nil {
		RefreshCloneResources(s)
	}
}

// isCloneTool reports whether a completed tool call should trigger a resource
// refresh. The mirrored `clone` command is the only tool that adds sections to
// the store; every other tool is read-only or leaves the section set unchanged.
func isCloneTool(name string) bool {
	return name == "clone"
}

// buildCloneResources enumerates the inventory resource plus one resource per
// cloned section. Returns just the inventory resource when the store is
// missing or empty (never an error, never a network call).
func buildCloneResources() []server.ServerResource {
	out := []server.ServerResource{{
		Resource: mcplib.NewResource(
			clonesURI,
			"Cloned municipal codes",
			mcplib.WithResourceDescription("Inventory of locally cloned cities: state, ids, codification version, section count, last synced."),
			mcplib.WithMIMEType("application/json"),
		),
		Handler: handleClonesResource,
	}}

	path, err := mcpDBPath()
	if err != nil {
		return out
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return out
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return out
	}
	defer db.Close()

	sections, err := mcpListSections(context.Background(), db)
	if err != nil {
		return out
	}
	for _, sec := range sections {
		desc := fmt.Sprintf("depth %d", sec.Depth)
		if sec.ParentID != "" {
			desc += "; parent " + sec.ParentID
		}
		out = append(out, server.ServerResource{
			Resource: mcplib.NewResource(
				sectionURI(sec.ClientID, sec.NodeID),
				sec.Title,
				mcplib.WithResourceDescription(desc),
				mcplib.WithMIMEType("text/plain"),
			),
			Handler: handleSectionResource,
		})
	}
	return out
}

func sectionURI(clientID int, nodeID string) string {
	return fmt.Sprintf("%s%d/%s", sectionURIPrefix, clientID, nodeID)
}

// parseSectionURI extracts clientId and nodeId from a section resource URI.
func parseSectionURI(uri string) (clientID int, nodeID string, ok bool) {
	rest := strings.TrimPrefix(uri, sectionURIPrefix)
	if rest == uri {
		return 0, "", false
	}
	slash := strings.IndexByte(rest, '/')
	if slash <= 0 || slash == len(rest)-1 {
		return 0, "", false
	}
	id, err := strconv.Atoi(rest[:slash])
	if err != nil {
		return 0, "", false
	}
	return id, rest[slash+1:], true
}

// --- Resource handlers ---

func handleClonesResource(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	inv, err := mcpCloneInventory(ctx)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		return nil, err
	}
	return []mcplib.ResourceContents{mcplib.TextResourceContents{
		URI:      clonesURI,
		MIMEType: "application/json",
		Text:     string(data),
	}}, nil
}

func handleSectionResource(ctx context.Context, req mcplib.ReadResourceRequest) ([]mcplib.ResourceContents, error) {
	clientID, nodeID, ok := parseSectionURI(req.Params.URI)
	if !ok {
		return nil, fmt.Errorf("invalid section resource URI %q (want %s)", req.Params.URI, sectionURITemplate)
	}
	path, err := mcpDBPath()
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return nil, fmt.Errorf("no local clone at %s; run: municode-pp-cli clone \"<City, ST>\"", path)
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	title, text, citation, found, err := mcpReadSection(ctx, db, clientID, nodeID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("section %s not found in the local clone of client %d", nodeID, clientID)
	}
	body := text
	if title != "" {
		body = title + "\n\n" + text
	}
	if citation != "" {
		body += "\n\nSource: " + citation
	}
	return []mcplib.ResourceContents{mcplib.TextResourceContents{
		URI:      req.Params.URI,
		MIMEType: "text/plain",
		Text:     body,
	}}, nil
}

// mcpCloneRow is one cloned municipality in the inventory. Field set matches
// the CLI `clones` command output (parity).
type mcpCloneRow struct {
	City       string `json:"city"`
	State      string `json:"state"`
	ClientID   int    `json:"client_id"`
	ProductID  int    `json:"product_id"`
	JobID      int    `json:"job_id"`
	Sections   int    `json:"sections"`
	LastSynced string `json:"last_synced"`
}

// mcpCloneInventory lists the distinct cloned municipalities in the default
// store. Empty (not error) when no clone exists.
func mcpCloneInventory(ctx context.Context) ([]mcpCloneRow, error) {
	out := make([]mcpCloneRow, 0)
	path, err := mcpDBPath()
	if err != nil {
		return out, nil
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return out, nil
	}
	db, err := store.OpenReadOnly(path)
	if err != nil {
		return out, nil
	}
	defer db.Close()

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
		ORDER BY json_extract(data,'$.client')`, mcpDocType)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var (
			cid, pid, jid, n  sql.NullInt64
			client, state, ts sql.NullString
		)
		if err := rows.Scan(&cid, &client, &state, &pid, &jid, &n, &ts); err != nil {
			_ = rows.Close()
			return out, err
		}
		out = append(out, mcpCloneRow{
			City:       client.String,
			State:      state.String,
			ClientID:   int(cid.Int64),
			ProductID:  int(pid.Int64),
			JobID:      int(jid.Int64),
			Sections:   int(n.Int64),
			LastSynced: ts.String,
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, err
	}
	return out, rows.Close()
}

// --- Store queries (direct, package-local; pinned to the default store) ---

type mcpSectionRow struct {
	ClientID int
	NodeID   string
	Title    string
	ParentID string
	Depth    int
}

func mcpListSections(ctx context.Context, db *store.Store) ([]mcpSectionRow, error) {
	rows, err := db.DB().QueryContext(ctx, `
		SELECT json_extract(data,'$.client_id'),
		       json_extract(data,'$.node_id'),
		       json_extract(data,'$.title'),
		       json_extract(data,'$.parent_id'),
		       json_extract(data,'$.depth')
		FROM resources WHERE resource_type = ?
		ORDER BY json_extract(data,'$.client_id'), json_extract(data,'$.depth')`, mcpDocType)
	if err != nil {
		return nil, err
	}
	out := make([]mcpSectionRow, 0)
	for rows.Next() {
		var (
			cid, depth  sql.NullInt64
			node, title sql.NullString
			parent      sql.NullString
		)
		if err := rows.Scan(&cid, &node, &title, &parent, &depth); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if node.String == "" {
			continue
		}
		out = append(out, mcpSectionRow{
			ClientID: int(cid.Int64),
			NodeID:   node.String,
			Title:    title.String,
			ParentID: parent.String,
			Depth:    int(depth.Int64),
		})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

// mcpReadSection returns the stored title/text/citation for one cloned section.
func mcpReadSection(ctx context.Context, db *store.Store, clientID int, nodeID string) (title, text, citation string, found bool, err error) {
	var data sql.NullString
	row := db.DB().QueryRowContext(ctx, `
		SELECT data FROM resources
		WHERE resource_type = ? AND json_extract(data,'$.client_id') = ? AND json_extract(data,'$.node_id') = ?
		LIMIT 1`, mcpDocType, clientID, nodeID)
	switch scanErr := row.Scan(&data); scanErr {
	case sql.ErrNoRows:
		return "", "", "", false, nil
	case nil:
	default:
		return "", "", "", false, scanErr
	}
	if !data.Valid {
		return "", "", "", false, nil
	}
	var rec struct {
		Title    string `json:"title"`
		Text     string `json:"text"`
		Citation string `json:"citation"`
	}
	if err := json.Unmarshal([]byte(data.String), &rec); err != nil {
		return "", "", "", false, err
	}
	return rec.Title, rec.Text, rec.Citation, true, nil
}
