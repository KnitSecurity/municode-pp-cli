// Copyright 2026 Clu and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored Municode API + parsing helpers shared by the friendly and
// transcendence commands. Not generated; preserved across regen.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"municode-pp-cli/internal/client"
	"municode-pp-cli/internal/cliutil"
)

// Resource type used for synced section documents in the generic store.
const mcDocType = "document"

// --- API response structs (only the fields we consume) ---

type mcState struct {
	StateID           int    `json:"StateID"`
	StateName         string `json:"StateName"`
	StateAbbreviation string `json:"StateAbbreviation"`
}

type mcClient struct {
	ClientID   int    `json:"ClientID"`
	ClientName string `json:"ClientName"`
	City       string `json:"City"`
	Address    string `json:"Address"`
	Website    string `json:"Website"`
	State      struct {
		StateAbbreviation string `json:"StateAbbreviation"`
		StateName         string `json:"StateName"`
	} `json:"State"`
}

type mcProduct struct {
	ProductID   int    `json:"ProductID"`
	ProductName string `json:"ProductName"`
}

type mcJob struct {
	Id int `json:"Id"`
}

type mcTocNode struct {
	Id      string `json:"Id"`
	Heading string `json:"Heading"`
}

type mcDoc struct {
	Id        string `json:"Id"`
	TitleHtml string `json:"TitleHtml"`
	Content   string `json:"Content"`
	NodeDepth int    `json:"NodeDepth"`
}

type mcContentResp struct {
	Docs []mcDoc `json:"Docs"`
}

// mcResolved is the addressable handle for a municipality's code.
type mcResolved struct {
	ClientID    int    `json:"client_id"`
	ClientName  string `json:"client"`
	StateAbbr   string `json:"state"`
	City        string `json:"city"`
	Website     string `json:"website,omitempty"`
	ProductID   int    `json:"product_id"`
	ProductName string `json:"product"`
	JobID       int    `json:"job_id"`
	LibraryURL  string `json:"library_url"`
}

// mcParseCity splits "Atlanta, GA" into ("atlanta", "GA").
func mcParseCity(s string) (name, abbr string, err error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("municipality must be in \"City, ST\" form, got %q", s)
	}
	name = strings.TrimSpace(parts[0])
	abbr = strings.ToUpper(strings.TrimSpace(parts[1]))
	if name == "" || len(abbr) != 2 {
		return "", "", fmt.Errorf("municipality must be in \"City, ST\" form, got %q", s)
	}
	return name, abbr, nil
}

func mcSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// mcLibraryURL builds the public library permalink for a code (optionally a node).
func mcLibraryURL(abbr, clientName, nodeID string) string {
	u := fmt.Sprintf("https://library.municode.com/%s/%s/codes/code_of_ordinances",
		strings.ToLower(abbr), mcSlug(clientName))
	if nodeID != "" {
		u += "?nodeId=" + nodeID
	}
	return u
}

// mcResolve runs the State -> Client -> Product -> Job chain for "City, ST".
func mcResolve(ctx context.Context, c *client.Client, cityState string) (*mcResolved, error) {
	name, abbr, err := mcParseCity(cityState)
	if err != nil {
		return nil, err
	}
	raw, err := c.Get(ctx, "/Clients/name", map[string]string{"clientName": name, "stateAbbr": abbr})
	if err != nil {
		return nil, fmt.Errorf("resolving municipality %q: %w", cityState, err)
	}
	var cl mcClient
	if err := json.Unmarshal(raw, &cl); err != nil || cl.ClientID == 0 {
		return nil, fmt.Errorf("municipality %q not found on Municode", cityState)
	}
	prods, err := c.Get(ctx, "/Products/clientId/"+strconv.Itoa(cl.ClientID), nil)
	if err != nil {
		return nil, fmt.Errorf("listing products for %q: %w", cityState, err)
	}
	var products []mcProduct
	_ = json.Unmarshal(prods, &products)
	if len(products) == 0 {
		return nil, fmt.Errorf("no published code found for %q", cityState)
	}
	prod := products[0]
	for _, p := range products {
		n := strings.ToLower(p.ProductName)
		if strings.Contains(n, "code") || strings.Contains(n, "ordinance") {
			prod = p
			break
		}
	}
	jobID, err := mcLatestJob(ctx, c, prod.ProductID)
	if err != nil {
		return nil, err
	}
	return &mcResolved{
		ClientID:    cl.ClientID,
		ClientName:  cl.ClientName,
		StateAbbr:   abbr,
		City:        cl.City,
		Website:     cl.Website,
		ProductID:   prod.ProductID,
		ProductName: prod.ProductName,
		JobID:       jobID,
		LibraryURL:  mcLibraryURL(abbr, cl.ClientName, ""),
	}, nil
}

func mcLatestJob(ctx context.Context, c *client.Client, productID int) (int, error) {
	raw, err := c.Get(ctx, "/Jobs/latest/"+strconv.Itoa(productID), nil)
	if err != nil {
		return 0, fmt.Errorf("fetching latest job for product %d: %w", productID, err)
	}
	var j mcJob
	if err := json.Unmarshal(raw, &j); err != nil || j.Id == 0 {
		return 0, fmt.Errorf("no codification version available for product %d", productID)
	}
	return j.Id, nil
}

func mcTocChildren(ctx context.Context, c *client.Client, productID, jobID int, nodeID string) ([]mcTocNode, error) {
	raw, err := c.Get(ctx, "/codesToc/children", map[string]string{
		"productId": strconv.Itoa(productID),
		"jobId":     strconv.Itoa(jobID),
		"nodeId":    nodeID,
	})
	if err != nil {
		return nil, err
	}
	var nodes []mcTocNode
	_ = json.Unmarshal(raw, &nodes)
	return nodes, nil
}

func mcContent(ctx context.Context, c *client.Client, productID, jobID int, nodeID string) ([]mcDoc, error) {
	raw, err := c.Get(ctx, "/CodesContent", map[string]string{
		"productId": strconv.Itoa(productID),
		"jobId":     strconv.Itoa(jobID),
		"nodeId":    nodeID,
	})
	if err != nil {
		return nil, err
	}
	var resp mcContentResp
	_ = json.Unmarshal(raw, &resp)
	return resp.Docs, nil
}

var (
	mcTagRE = regexp.MustCompile(`<[^>]*>`)
	mcWSRE  = regexp.MustCompile(`\s+`)
)

// mcHTMLToText strips HTML tags, decodes entities, and collapses whitespace
// to clean single-spaced plaintext.
func mcHTMLToText(html string) string {
	withSpaces := mcTagRE.ReplaceAllString(html, " ")
	cleaned := cliutil.CleanText(withSpaces)
	return strings.TrimSpace(mcWSRE.ReplaceAllString(cleaned, " "))
}

// --- Ordinance-history annotation parsing ---

type mcOrdRef struct {
	Ordinance string `json:"ordinance,omitempty"`
	CodeYear  string `json:"code_year,omitempty"`
	Section   string `json:"section,omitempty"`
	Date      string `json:"date,omitempty"`
	Raw       string `json:"raw"`
}

var (
	// Matches both abbreviated ("Ord. No. 2006-45", Atlanta/Acworth) and
	// spelled-out ("Ordinance No. 4705", Boulder) forms, capturing the number
	// plus a short trailing context to pick up a section and/or date.
	mcOrdRE  = regexp.MustCompile(`(?:Ord\.|Ordinance)\s*No\.\s*([0-9][0-9A-Za-z()\-/]*)((?:[^.;()]|\([^()]*\)){0,50})`)
	mcCodeRE = regexp.MustCompile(`(?:Code\s+((?:19|20)\d{2})|((?:19|20)\d{2})\s+Code)`)
	mcDateRE = regexp.MustCompile(`\b(\d{1,2}-\d{1,2}-\d{2,4})\b`)
	mcSecRE  = regexp.MustCompile(`§+\s*([0-9][0-9A-Za-z.\-]*)`)
)

// mcParseOrdHistory extracts ordinance/codification history references from
// section plaintext. It handles both Municode annotation styles: the
// abbreviated trailing-parenthetical form ("(Code 1977, § 1-1; Ord. No.
// 2006-45, § 1, 7-25-06)") and the spelled-out footnote form ("Adopted by
// Ordinance No. 4705. Derived from Ordinance No. 3838, 1925 Code.").
func mcParseOrdHistory(text string) []mcOrdRef {
	var refs []mcOrdRef
	seen := map[string]bool{}
	for _, m := range mcOrdRE.FindAllStringSubmatch(text, -1) {
		ord := m[1]
		if seen["ord:"+ord] {
			continue
		}
		seen["ord:"+ord] = true
		ref := mcOrdRef{Ordinance: ord, Raw: strings.TrimSpace(m[0])}
		tail := m[2]
		if d := mcDateRE.FindString(tail); d != "" {
			ref.Date = d
		}
		if s := mcSecRE.FindStringSubmatch(tail); s != nil {
			ref.Section = s[1]
		}
		refs = append(refs, ref)
	}
	for _, m := range mcCodeRE.FindAllStringSubmatch(text, -1) {
		yr := m[1]
		if yr == "" {
			yr = m[2]
		}
		if yr == "" || seen["code:"+yr] {
			continue
		}
		seen["code:"+yr] = true
		refs = append(refs, mcOrdRef{CodeYear: yr, Raw: strings.TrimSpace(m[0])})
	}
	return refs
}

// --- Cross-reference parsing ---

var mcXrefRE = regexp.MustCompile(`(?:§+\s*[0-9][0-9A-Za-z.\-]*|Chapter\s+\d+[A-Za-z]?|Article\s+[IVXLC]+|Sec(?:tion)?\.?\s*[0-9][0-9A-Za-z.\-]*)`)

// mcSecCiteRE captures the full section-number token from a citation so an
// exact comparison can distinguish "1-2" from "1-20", "1-2-09", etc.
var mcSecCiteRE = regexp.MustCompile(`(?i)(?:§+|sec(?:tion)?\.?)\s*([0-9][0-9A-Za-z.\-]*)`)

// mcMentionsSection reports whether text cites the given section as a complete
// token (e.g. "§ 1-2" matches "1-2" but "§ 1-20" and "§ 1-2-09" do not).
func mcMentionsSection(text, section string) bool {
	section = strings.TrimSpace(section)
	if section == "" {
		return false
	}
	for _, m := range mcSecCiteRE.FindAllStringSubmatch(text, -1) {
		if strings.EqualFold(strings.TrimRight(m[1], "."), section) {
			return true
		}
	}
	return false
}

// mcParseXrefs extracts intra-code references mentioned in section text.
func mcParseXrefs(text string) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range mcXrefRE.FindAllString(text, -1) {
		key := strings.Join(strings.Fields(m), " ")
		if !seen[key] {
			seen[key] = true
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}
