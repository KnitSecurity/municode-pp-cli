// Copyright 2026 Ryan Jamieson and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored. MuniDocs API: the separate "documents" product that serves
// Rules (and, later, Ordinances) as PDFs rather than inline HTML. The tree comes
// from /munidocsToc; each leaf's PDF is a public download from Municode's Azure
// Functions host. See docs/local-clone-mcp.md.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/KnitSecurity/municode-pp-cli/internal/client"
)

// mcMuniDocsFuncBase is Municode's public Azure Functions host that streams the
// original PDF for a MuniDocs node (no auth; deterministic path).
const mcMuniDocsFuncBase = "https://mcclibraryfunctions.azurewebsites.us/api"

// mcRulesRootNode is the MuniDocs tree root that holds the Rules folders.
const mcRulesRootNode = "rules"

// mcTocNode is reused from municode_api.go (Id/Heading). MuniDocs adds folder and
// PDF metadata, captured here.
type mcMuniDocNode struct {
	Id          string `json:"Id"`
	Heading     string `json:"Heading"`
	HasChildren bool   `json:"HasChildren"`
	ParentId    string `json:"ParentId"`
	Data        struct {
		IsFolder bool `json:"IsFolder"`
	} `json:"Data"`
}

type mcMuniDocMeta struct {
	Id               string `json:"Id"`
	NodeId           string `json:"NodeId"`
	Heading          string `json:"Heading"`
	OriginalFileName string `json:"OriginalFileName"`
	IsOriginalAPdf   bool   `json:"IsOriginalAPdf"`
	SortDate         string `json:"SortDate"`
	Ancestors        []struct {
		Id      string `json:"Id"`
		Heading string `json:"Heading"`
	} `json:"Ancestors"`
}

// mcMuniDocsProductID finds the MUNIDOCS product for a client (the Rules/docs
// product, distinct from the Code of Ordinances product).
func mcMuniDocsProductID(ctx context.Context, c *client.Client, clientID int) (int, error) {
	raw, err := c.Get(ctx, "/Products/clientId/"+strconv.Itoa(clientID), nil)
	if err != nil {
		return 0, fmt.Errorf("listing products: %w", err)
	}
	var products []struct {
		ProductID   int `json:"ProductID"`
		ContentType struct {
			Id string `json:"Id"`
		} `json:"ContentType"`
	}
	_ = json.Unmarshal(raw, &products)
	for _, p := range products {
		if p.ContentType.Id == "MUNIDOCS" {
			return p.ProductID, nil
		}
	}
	return 0, fmt.Errorf("no MuniDocs product for client %d (city may not host Rules/ordinance PDFs)", clientID)
}

// mcMuniDocsChildren lists the child nodes (folders + leaf PDFs) of a MuniDocs node.
func mcMuniDocsChildren(ctx context.Context, c *client.Client, productID int, nodeID string) ([]mcMuniDocNode, error) {
	raw, err := c.Get(ctx, "/munidocsToc/children", map[string]string{
		"productId": strconv.Itoa(productID),
		"nodeId":    nodeID,
	})
	if err != nil {
		return nil, err
	}
	var nodes []mcMuniDocNode
	_ = json.Unmarshal(raw, &nodes)
	return nodes, nil
}

// mcMuniDocMetadata fetches a leaf document's metadata (filename, is-pdf, date,
// breadcrumb). Content is never inline for these — the body is the PDF.
func mcMuniDocMetadata(ctx context.Context, c *client.Client, productID int, nodeID string) (*mcMuniDocMeta, error) {
	raw, err := c.Get(ctx, "/MuniDocsContent", map[string]string{
		"productId": strconv.Itoa(productID),
		"nodeId":    nodeID,
	})
	if err != nil {
		return nil, err
	}
	var m mcMuniDocMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// mcMuniDocPDFURL builds the public PDF download URL for a Rules/MuniDocs node.
func mcMuniDocPDFURL(productID int, nodeID string) string {
	return fmt.Sprintf("%s/munidocDownload/%d/%s/pdf", mcMuniDocsFuncBase, productID, nodeID)
}

// --- Ordinances (the code product's OrdBank; PDFs organized by adoption year) ---

// mcOrdinanceMeta is the metadata for a single ordinance.
type mcOrdinanceMeta struct {
	Id           int    `json:"Id"`
	Title        string `json:"Title"`
	Description  string `json:"Description"`
	AdoptionDate string `json:"AdoptionDate"`
}

// mcOrdinanceYears returns the year folders under a code product's ordinances
// tree (the OrdBank root's direct children).
func mcOrdinanceYears(ctx context.Context, c *client.Client, productID int) ([]mcMuniDocNode, error) {
	raw, err := c.Get(ctx, "/ordinancesToc", map[string]string{"productId": strconv.Itoa(productID)})
	if err != nil {
		return nil, err
	}
	var root struct {
		Children []mcMuniDocNode `json:"Children"`
	}
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	return root.Children, nil
}

// mcOrdinancesInYear lists the ordinance leaves under a year node.
func mcOrdinancesInYear(ctx context.Context, c *client.Client, productID int, yearNodeID string) ([]mcMuniDocNode, error) {
	raw, err := c.Get(ctx, "/ordinancesToc/children", map[string]string{
		"productId": strconv.Itoa(productID),
		"nodeId":    yearNodeID,
	})
	if err != nil {
		return nil, err
	}
	var nodes []mcMuniDocNode
	_ = json.Unmarshal(raw, &nodes)
	return nodes, nil
}

// mcOrdinanceMetadata fetches an ordinance's title/subject/adoption date.
func mcOrdinanceMetadata(ctx context.Context, c *client.Client, productID int, ordID string) (*mcOrdinanceMeta, error) {
	raw, err := c.Get(ctx, "/CoreContent/Ordinances", map[string]string{
		"productId": strconv.Itoa(productID),
		"nodeId":    ordID,
	})
	if err != nil {
		return nil, err
	}
	var m mcOrdinanceMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// mcOrdinancePDFURL builds the public PDF download URL for an ordinance.
func mcOrdinancePDFURL(productID int, ordID string) string {
	return fmt.Sprintf("%s/ordinanceDownload/%d/%s/pdf", mcMuniDocsFuncBase, productID, ordID)
}

// mcFetchPDF downloads PDF bytes from an absolute URL (the Azure Functions host,
// not the API base) with a small bounded retry so a transient blip doesn't drop
// a document mid-clone. Honors ctx cancellation.
func mcFetchPDF(ctx context.Context, url string, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	httpc := &http.Client{Timeout: timeout}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt > 0 {
			if err := sleepCtx(ctx, time.Duration(1<<uint(attempt-1))*time.Second); err != nil {
				return nil, err
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := httpc.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			lastErr = fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
			if resp.StatusCode < 500 {
				return nil, lastErr // client error: don't retry
			}
			continue
		}
		if readErr != nil {
			lastErr = readErr
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

// sleepCtx sleeps for d or returns early if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
