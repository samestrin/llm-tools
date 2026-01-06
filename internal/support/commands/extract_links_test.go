package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestExtractLinksCommand_MissingURL(t *testing.T) {
	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
}

func TestExtractLinksCommand_InvalidURL(t *testing.T) {
	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--url", "not-a-url"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("expected 'invalid URL' error, got: %v", err)
	}
}

func TestExtractLinksCommand_Flags(t *testing.T) {
	cmd := newExtractLinksCmd()

	if cmd.Flag("url") == nil {
		t.Error("expected --url flag")
	}
	if cmd.Flag("timeout") == nil {
		t.Error("expected --timeout flag")
	}
	if cmd.Flag("json") == nil {
		t.Error("expected --json flag")
	}
	if cmd.Flag("min") == nil {
		t.Error("expected --min flag")
	}

	// Check default timeout
	timeoutFlag := cmd.Flag("timeout")
	if timeoutFlag.DefValue != "30" {
		t.Errorf("expected default timeout 30, got %s", timeoutFlag.DefValue)
	}
}

func TestExtractLinksFromHTML_Basic(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Main Title</h1>
<p>Some text with a <a href="/docs">documentation link</a>.</p>
<h2>Section One</h2>
<p>Another <a href="/api">API link</a> here.</p>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 2 {
		t.Fatalf("expected 2 links, got %d", len(links))
	}

	// Links should be sorted by score (highest first)
	// Both are in <p> tags, so same base score
	for _, link := range links {
		if link.Href == "" {
			t.Error("link href should not be empty")
		}
		if link.Text == "" {
			t.Error("link text should not be empty")
		}
		if link.Score == 0 {
			t.Error("link score should not be zero")
		}
	}
}

func TestExtractLinksFromHTML_Scoring(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<h1><a href="/h1-link">H1 Link</a></h1>
<h2><a href="/h2-link">H2 Link</a></h2>
<nav><a href="/nav-link">Nav Link</a></nav>
<footer><a href="/footer-link">Footer Link</a></footer>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 4 {
		t.Fatalf("expected 4 links, got %d", len(links))
	}

	// Should be sorted by score: h1 > h2 > nav > footer
	expectedOrder := []string{"/h1-link", "/h2-link", "/nav-link", "/footer-link"}
	for i, expected := range expectedOrder {
		if !strings.HasSuffix(links[i].Href, expected) {
			t.Errorf("link %d: expected href ending with %s, got %s", i, expected, links[i].Href)
		}
	}

	// Verify specific scores
	scoreMap := map[string]int{
		"/h1-link":     100,
		"/h2-link":     85,
		"/nav-link":    30,
		"/footer-link": 10,
	}
	for _, link := range links {
		for suffix, expectedScore := range scoreMap {
			if strings.HasSuffix(link.Href, suffix) {
				if link.Score != expectedScore {
					t.Errorf("link %s: expected score %d, got %d", suffix, expectedScore, link.Score)
				}
			}
		}
	}
}

func TestExtractLinksFromHTML_Modifiers(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<p><a href="/plain">Plain Link</a></p>
<p><strong><a href="/bold">Bold Link</a></strong></p>
<p><em><a href="/em">Emphasized Link</a></em></p>
<p><a href="/with-title" title="Has Title">Title Link</a></p>
<p><a href="/button" role="button">Button Link</a></p>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 5 {
		t.Fatalf("expected 5 links, got %d", len(links))
	}

	// Find each link and verify modifiers add to score
	scores := make(map[string]int)
	for _, link := range links {
		for _, suffix := range []string{"/plain", "/bold", "/em", "/with-title", "/button"} {
			if strings.HasSuffix(link.Href, suffix) {
				scores[suffix] = link.Score
			}
		}
	}

	// Base score for <p> is 40
	if scores["/plain"] != 40 {
		t.Errorf("plain link: expected score 40, got %d", scores["/plain"])
	}
	if scores["/bold"] != 55 { // 40 + 15
		t.Errorf("bold link: expected score 55, got %d", scores["/bold"])
	}
	if scores["/em"] != 50 { // 40 + 10
		t.Errorf("em link: expected score 50, got %d", scores["/em"])
	}
	if scores["/with-title"] != 45 { // 40 + 5
		t.Errorf("title link: expected score 45, got %d", scores["/with-title"])
	}
	if scores["/button"] != 50 { // 40 + 10
		t.Errorf("button link: expected score 50, got %d", scores["/button"])
	}
}

func TestExtractLinksFromHTML_Sections(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<h1>Main Title</h1>
<p><a href="/intro">Intro Link</a></p>
<h2>Getting Started</h2>
<p><a href="/getting-started">Getting Started Link</a></p>
<h3>Installation</h3>
<p><a href="/install">Install Link</a></p>
<h2>API Reference</h2>
<p><a href="/api">API Link</a></p>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	// Check sections are tracked correctly
	sectionMap := make(map[string]string)
	for _, link := range links {
		for _, suffix := range []string{"/intro", "/getting-started", "/install", "/api"} {
			if strings.HasSuffix(link.Href, suffix) {
				sectionMap[suffix] = link.Section
			}
		}
	}

	if sectionMap["/intro"] != "Main Title" {
		t.Errorf("intro link section: expected 'Main Title', got '%s'", sectionMap["/intro"])
	}
	if sectionMap["/getting-started"] != "Getting Started" {
		t.Errorf("getting-started link section: expected 'Getting Started', got '%s'", sectionMap["/getting-started"])
	}
	if sectionMap["/install"] != "Installation" {
		t.Errorf("install link section: expected 'Installation', got '%s'", sectionMap["/install"])
	}
	if sectionMap["/api"] != "API Reference" {
		t.Errorf("api link section: expected 'API Reference', got '%s'", sectionMap["/api"])
	}
}

func TestExtractLinksFromHTML_RelativeURLs(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<a href="/absolute">Absolute</a>
<a href="relative">Relative</a>
<a href="../parent">Parent</a>
<a href="https://other.com/external">External</a>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com/docs/page")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 4 {
		t.Fatalf("expected 4 links, got %d", len(links))
	}

	expectedHrefs := map[string]bool{
		"https://example.com/absolute":      true,
		"https://example.com/docs/relative": true,
		"https://example.com/parent":        true,
		"https://other.com/external":        true,
	}

	for _, link := range links {
		if !expectedHrefs[link.Href] {
			t.Errorf("unexpected href: %s", link.Href)
		}
	}
}

func TestExtractLinksFromHTML_SkipsInvalid(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<a href="/valid">Valid</a>
<a href="#">Anchor</a>
<a href="javascript:alert('hi')">JavaScript</a>
<a href="mailto:test@example.com">Email</a>
<a>No href</a>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 valid link, got %d", len(links))
	}

	if !strings.HasSuffix(links[0].Href, "/valid") {
		t.Errorf("expected valid link, got %s", links[0].Href)
	}
}

func TestExtractLinksFromHTML_Deduplication(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<a href="/docs">Docs 1</a>
<a href="/docs">Docs 2</a>
<a href="/docs">Docs 3</a>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 deduplicated link, got %d", len(links))
	}
}

func TestExtractLinksFromHTML_ImageAltText(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<a href="/image-link"><img src="logo.png" alt="Company Logo"></a>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(links))
	}

	if links[0].Text != "Company Logo" {
		t.Errorf("expected text 'Company Logo' from alt, got '%s'", links[0].Text)
	}
}

func TestExtractLinksCommand_JSONOutput(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<h1>Test</h1>
<a href="/link1">Link One</a>
<a href="/link2">Link Two</a>
</body>
</html>`))
	}))
	defer server.Close()

	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", server.URL, "--json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var result ExtractLinksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.URL != server.URL {
		t.Errorf("expected URL %s, got %s", server.URL, result.URL)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 links, got %d", result.Total)
	}
	if len(result.Links) != 2 {
		t.Errorf("expected 2 links in array, got %d", len(result.Links))
	}
}

func TestExtractLinksCommand_MinimalOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<a href="/test">Test Link</a>
</body>
</html>`))
	}))
	defer server.Close()

	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", server.URL, "--json", "--min"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	var result ExtractLinksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Minimal output uses short keys
	if result.U != server.URL {
		t.Errorf("expected U (url) %s, got %s", server.URL, result.U)
	}
	if result.N == nil || *result.N != 1 {
		t.Errorf("expected N (total) 1")
	}
	if len(result.L) != 1 {
		t.Errorf("expected 1 link in L array, got %d", len(result.L))
	}
	if result.L[0].H == "" {
		t.Error("expected H (href) to be set in minimal output")
	}
}

func TestExtractLinksCommand_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", server.URL, "--json"})

	err := cmd.Execute()
	// Command should not error, but result should contain error
	if err != nil {
		t.Fatalf("command should not fail, error should be in result: %v", err)
	}

	var result ExtractLinksResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.Error == "" {
		t.Error("expected error in result for 404 response")
	}
}

func TestGetParentContext(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<body>
<nav><a id="nav-link" href="/nav">Nav</a></nav>
<main><a id="main-link" href="/main">Main</a></main>
<article><a id="article-link" href="/article">Article</a></article>
<footer><a id="footer-link" href="/footer">Footer</a></footer>
<p><a id="p-link" href="/p">Paragraph</a></p>
<li><a id="li-link" href="/li">List Item</a></li>
</body>
</html>`

	links, err := extractLinksFromHTML(strings.NewReader(html), "https://example.com")
	if err != nil {
		t.Fatalf("extractLinksFromHTML failed: %v", err)
	}

	contextMap := make(map[string]string)
	for _, link := range links {
		for _, suffix := range []string{"/nav", "/main", "/article", "/footer", "/p", "/li"} {
			if strings.HasSuffix(link.Href, suffix) {
				contextMap[suffix] = link.Context
			}
		}
	}

	expected := map[string]string{
		"/nav":     "nav",
		"/main":    "main",
		"/article": "article",
		"/footer":  "footer",
		"/p":       "p",
		"/li":      "li",
	}

	for suffix, expectedContext := range expected {
		if contextMap[suffix] != expectedContext {
			t.Errorf("link %s: expected context '%s', got '%s'", suffix, expectedContext, contextMap[suffix])
		}
	}
}

func TestCalculateLinkScore(t *testing.T) {
	tests := []struct {
		context  string
		expected int
	}{
		{"h1", 100},
		{"h2", 85},
		{"h3", 70},
		{"h4", 55},
		{"h5", 55},
		{"h6", 55},
		{"main", 50},
		{"article", 50},
		{"p", 40},
		{"li", 35},
		{"nav", 30},
		{"header", 25},
		{"aside", 20},
		{"footer", 10},
		{"body", 30},
		{"unknown", 30}, // default
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			// Create a minimal selection for testing
			html := `<a href="/test">Test</a>`
			doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
			link := doc.Find("a").First()

			score := calculateLinkScore(link, tt.context)
			if score != tt.expected {
				t.Errorf("calculateLinkScore(%s) = %d, want %d", tt.context, score, tt.expected)
			}
		})
	}
}

func TestResolveURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/docs/page")

	tests := []struct {
		href     string
		expected string
	}{
		{"/absolute", "https://example.com/absolute"},
		{"relative", "https://example.com/docs/relative"},
		{"../parent", "https://example.com/parent"},
		{"https://other.com/external", "https://other.com/external"},
		{"//cdn.example.com/resource", "https://cdn.example.com/resource"},
	}

	for _, tt := range tests {
		t.Run(tt.href, func(t *testing.T) {
			result := resolveURL(tt.href, base)
			if result != tt.expected {
				t.Errorf("resolveURL(%s) = %s, want %s", tt.href, result, tt.expected)
			}
		})
	}
}

func TestSortLinksByScore(t *testing.T) {
	links := []LinkInfo{
		{Href: "/low", Score: 10},
		{Href: "/high", Score: 100},
		{Href: "/mid", Score: 50},
	}

	sortLinksByScore(links)

	if links[0].Score != 100 {
		t.Errorf("first link should have highest score, got %d", links[0].Score)
	}
	if links[1].Score != 50 {
		t.Errorf("second link should have middle score, got %d", links[1].Score)
	}
	if links[2].Score != 10 {
		t.Errorf("third link should have lowest score, got %d", links[2].Score)
	}
}

func TestLinkInfoJSON(t *testing.T) {
	link := LinkInfo{
		Href:    "https://example.com/docs",
		Text:    "Documentation",
		Context: "h2",
		Score:   85,
		Section: "Getting Started",
	}

	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed LinkInfo
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Href != link.Href {
		t.Errorf("Href mismatch: got %s, want %s", parsed.Href, link.Href)
	}
	if parsed.Section != link.Section {
		t.Errorf("Section mismatch: got %s, want %s", parsed.Section, link.Section)
	}
}

func TestExtractLinksCommand_TextOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<h1>Test Page</h1>
<h2>Getting Started</h2>
<p><a href="/docs">Documentation</a></p>
</body>
</html>`))
	}))
	defer server.Close()

	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", server.URL}) // No --json flag = text output

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "URL:") {
		t.Error("expected 'URL:' in text output")
	}
	if !strings.Contains(output, "Total Links:") {
		t.Error("expected 'Total Links:' in text output")
	}
	if !strings.Contains(output, "Documentation") {
		t.Error("expected link text in output")
	}
	if !strings.Contains(output, "Section:") || !strings.Contains(output, "Context:") {
		t.Error("expected 'Section:' and 'Context:' in text output")
	}
}

func TestExtractLinksCommand_TextOutputNoSection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<!DOCTYPE html>
<html>
<body>
<p><a href="/simple">Simple Link</a></p>
</body>
</html>`))
	}))
	defer server.Close()

	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", server.URL})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command failed: %v", err)
	}

	output := buf.String()
	// When no section, should just show context
	if !strings.Contains(output, "Context:") {
		t.Error("expected 'Context:' in text output")
	}
}

func TestExtractLinksCommand_TextOutputError(t *testing.T) {
	// Test with unreachable URL
	cmd := newExtractLinksCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--url", "http://localhost:99999/unreachable"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("command should not fail, error should be in output: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ERROR:") {
		t.Error("expected 'ERROR:' in text output for failed request")
	}
}

// LLM Ranking Tests

func TestParseLLMRankings_ValidJSON(t *testing.T) {
	response := `{"rankings":[{"index":0,"score":95},{"index":1,"score":45},{"index":2,"score":10}]}`

	rankings, err := parseLLMRankings(response, 3)
	if err != nil {
		t.Fatalf("parseLLMRankings failed: %v", err)
	}

	if len(rankings) != 3 {
		t.Fatalf("expected 3 rankings, got %d", len(rankings))
	}

	expected := []LLMRanking{
		{Index: 0, Score: 95},
		{Index: 1, Score: 45},
		{Index: 2, Score: 10},
	}

	for i, r := range rankings {
		if r.Index != expected[i].Index || r.Score != expected[i].Score {
			t.Errorf("ranking %d: expected %+v, got %+v", i, expected[i], r)
		}
	}
}

func TestParseLLMRankings_WithExtraText(t *testing.T) {
	// LLM sometimes adds text before/after JSON
	response := `Here are the rankings:
{"rankings":[{"index":0,"score":80},{"index":1,"score":20}]}

I've ranked these based on relevance.`

	rankings, err := parseLLMRankings(response, 2)
	if err != nil {
		t.Fatalf("parseLLMRankings failed: %v", err)
	}

	if len(rankings) != 2 {
		t.Fatalf("expected 2 rankings, got %d", len(rankings))
	}

	if rankings[0].Score != 80 {
		t.Errorf("expected first score 80, got %d", rankings[0].Score)
	}
}

func TestParseLLMRankings_ScoreClamping(t *testing.T) {
	// Scores outside 0-100 should be clamped
	response := `{"rankings":[{"index":0,"score":150},{"index":1,"score":-10}]}`

	rankings, err := parseLLMRankings(response, 2)
	if err != nil {
		t.Fatalf("parseLLMRankings failed: %v", err)
	}

	if rankings[0].Score != 100 {
		t.Errorf("expected score clamped to 100, got %d", rankings[0].Score)
	}
	if rankings[1].Score != 0 {
		t.Errorf("expected score clamped to 0, got %d", rankings[1].Score)
	}
}

func TestParseLLMRankings_InvalidJSON(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{"no json", "This is just text without any JSON"},
		{"malformed json", `{"rankings": [{"index": 0, "score": `},
		{"empty response", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseLLMRankings(tt.response, 1)
			if err == nil {
				t.Error("expected error for invalid JSON")
			}
		})
	}
}

func TestRankLinksWithLLM_EmptyLinks(t *testing.T) {
	links := []LinkInfo{}

	result, err := rankLinksWithLLM(links, "test context", 30)
	if err != nil {
		t.Fatalf("rankLinksWithLLM failed: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty result, got %d links", len(result))
	}
}

func TestLLMRankRequest_JSON(t *testing.T) {
	req := LLMRankRequest{
		Context: "authentication",
		Links: []LLMLinkInput{
			{Index: 0, Href: "https://example.com/auth", Text: "Auth Docs", Section: "Security"},
			{Index: 1, Href: "https://example.com/api", Text: "API Reference", Section: ""},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed LLMRankRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Context != "authentication" {
		t.Errorf("context mismatch: got %s", parsed.Context)
	}
	if len(parsed.Links) != 2 {
		t.Errorf("expected 2 links, got %d", len(parsed.Links))
	}
	if parsed.Links[0].Section != "Security" {
		t.Errorf("section mismatch: got %s", parsed.Links[0].Section)
	}
}

func TestLLMRankResponse_JSON(t *testing.T) {
	jsonStr := `{"rankings":[{"index":0,"score":95},{"index":1,"score":30}]}`

	var resp LLMRankResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(resp.Rankings) != 2 {
		t.Fatalf("expected 2 rankings, got %d", len(resp.Rankings))
	}
	if resp.Rankings[0].Index != 0 || resp.Rankings[0].Score != 95 {
		t.Errorf("first ranking mismatch: %+v", resp.Rankings[0])
	}
}

func TestExtractLinksCommand_ContextFlag(t *testing.T) {
	cmd := newExtractLinksCmd()

	if cmd.Flag("context") == nil {
		t.Error("expected --context flag")
	}

	contextFlag := cmd.Flag("context")
	if contextFlag.DefValue != "" {
		t.Errorf("expected default context to be empty, got %s", contextFlag.DefValue)
	}
}

func TestApplyLLMScoresToLinks(t *testing.T) {
	// Test that scores are properly applied and sorted
	links := []LinkInfo{
		{Href: "/first", Text: "First", Score: 50},
		{Href: "/second", Text: "Second", Score: 30},
		{Href: "/third", Text: "Third", Score: 70},
	}

	rankings := []LLMRanking{
		{Index: 0, Score: 10}, // First becomes lowest
		{Index: 1, Score: 90}, // Second becomes highest
		{Index: 2, Score: 50}, // Third stays middle
	}

	// Apply scores
	for _, ranking := range rankings {
		if ranking.Index >= 0 && ranking.Index < len(links) {
			links[ranking.Index].Score = ranking.Score
		}
	}

	// Sort by new scores
	sortLinksByScore(links)

	// Verify order: second (90) > third (50) > first (10)
	if links[0].Href != "/second" {
		t.Errorf("expected /second first, got %s", links[0].Href)
	}
	if links[1].Href != "/third" {
		t.Errorf("expected /third second, got %s", links[1].Href)
	}
	if links[2].Href != "/first" {
		t.Errorf("expected /first last, got %s", links[2].Href)
	}
}

func TestParseLLMRankings_OutOfBoundsIndex(t *testing.T) {
	// Rankings with out-of-bounds indices should still parse
	response := `{"rankings":[{"index":0,"score":80},{"index":99,"score":50}]}`

	rankings, err := parseLLMRankings(response, 2)
	if err != nil {
		t.Fatalf("parseLLMRankings failed: %v", err)
	}

	// Should parse both, even though index 99 is out of bounds
	if len(rankings) != 2 {
		t.Errorf("expected 2 rankings, got %d", len(rankings))
	}
}

func TestLLMLinkInput_OmitsEmptySection(t *testing.T) {
	link := LLMLinkInput{
		Index:   0,
		Href:    "https://example.com/test",
		Text:    "Test Link",
		Section: "",
	}

	data, err := json.Marshal(link)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Empty section should be omitted
	if strings.Contains(string(data), "section") {
		t.Error("expected empty section to be omitted from JSON")
	}
}
