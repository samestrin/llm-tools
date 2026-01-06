package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"

	"github.com/samestrin/llm-tools/pkg/llmapi"
	"github.com/samestrin/llm-tools/pkg/output"
)

var (
	extractLinksURL     string
	extractLinksContext string
	extractLinksTimeout int
	extractLinksJSON    bool
	extractLinksMinimal bool
)

// LinkInfo represents a single extracted link with context
type LinkInfo struct {
	Href    string `json:"href,omitempty"`
	H       string `json:"h,omitempty"`
	Text    string `json:"text,omitempty"`
	T       string `json:"t,omitempty"`
	Context string `json:"context,omitempty"`
	C       string `json:"c,omitempty"`
	Score   int    `json:"score,omitempty"`
	S       *int   `json:"s,omitempty"`
	Section string `json:"section,omitempty"`
	Sec     string `json:"sec,omitempty"`
}

// ExtractLinksResult holds the extraction result
type ExtractLinksResult struct {
	URL   string     `json:"url,omitempty"`
	U     string     `json:"u,omitempty"`
	Links []LinkInfo `json:"links,omitempty"`
	L     []LinkInfo `json:"l,omitempty"`
	Total int        `json:"total,omitempty"`
	N     *int       `json:"n,omitempty"`
	Error string     `json:"error,omitempty"`
	E     string     `json:"e,omitempty"`
}

// newExtractLinksCmd creates the extract-links command
func newExtractLinksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract-links",
		Short: "Extract and rank links from a URL",
		Long: `Extract links from a URL with intelligent ranking.

RANKING MODES:

1. Heuristic Mode (default - no --context flag):
   Links are scored based on their position in the document hierarchy:
     - h1: 100, h2: 85, h3: 70, h4-h6: 55
     - main/article: 50, p: 40, li: 35
     - nav: 30, aside: 20, footer: 10
   
   Modifiers add bonus points:
     - bold/strong: +15, em/i: +10
     - button role or .btn class: +10
     - has title attribute: +5

2. LLM Mode (when --context is provided):
   Uses an LLM to rank links by relevance to the specified context.
   Scores range from 0-100 based on semantic relevance.
   Requires OpenAI-compatible API configuration.

Each link includes its parent section heading when available.

Examples:
  # Heuristic ranking (default)
  llm-support extract-links --url https://example.com/docs
  llm-support extract-links --url https://example.com --json

  # LLM-based ranking
  llm-support extract-links --url https://example.com/docs --context "authentication"
  llm-support extract-links --url https://api.example.com --context "rate limiting" --json`,
		Args: cobra.NoArgs,
		RunE: runExtractLinks,
	}

	cmd.Flags().StringVar(&extractLinksURL, "url", "", "URL to extract links from (required)")
	cmd.Flags().StringVar(&extractLinksContext, "context", "", "Context for LLM-based ranking (enables LLM mode)")
	cmd.Flags().IntVar(&extractLinksTimeout, "timeout", 30, "HTTP timeout in seconds")
	cmd.Flags().BoolVar(&extractLinksJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&extractLinksMinimal, "min", false, "Output in minimal/token-optimized format")

	cmd.MarkFlagRequired("url")

	return cmd
}

func runExtractLinks(cmd *cobra.Command, args []string) error {
	if extractLinksURL == "" {
		return fmt.Errorf("--url is required")
	}

	// Validate URL
	if !isURL(extractLinksURL) {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	result := ExtractLinksResult{
		URL: extractLinksURL,
	}

	links, err := fetchAndExtractLinks(extractLinksURL, extractLinksTimeout)
	if err != nil {
		result.Error = err.Error()
	} else {
		// If context is provided, use LLM ranking instead of heuristic
		if extractLinksContext != "" {
			links, err = rankLinksWithLLM(links, extractLinksContext, extractLinksTimeout)
			if err != nil {
				result.Error = fmt.Sprintf("LLM ranking failed: %v", err)
				result.Links = links // Still include links even if LLM fails
				result.Total = len(links)
				return outputExtractLinksResult(cmd, result)
			}
		}
		result.Links = links
		result.Total = len(links)
	}

	return outputExtractLinksResult(cmd, result)
}

// fetchAndExtractLinks fetches a URL and extracts all links with ranking
func fetchAndExtractLinks(targetURL string, timeout int) ([]LinkInfo, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "llm-support/1.0 (extract-links)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.7")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %s", resp.Status)
	}

	return extractLinksFromHTML(resp.Body, targetURL)
}

// extractLinksFromHTML parses HTML and extracts links with context and scoring
func extractLinksFromHTML(r io.Reader, baseURL string) ([]LinkInfo, error) {
	doc, err := goquery.NewDocumentFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Parse base URL for resolving relative links
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	var links []LinkInfo
	seen := make(map[string]bool)

	// Track current section heading
	var currentSection string

	// Process the document in order to track sections
	doc.Find("h1, h2, h3, h4, h5, h6, a").Each(func(i int, s *goquery.Selection) {
		tagName := goquery.NodeName(s)

		// Update current section for headings
		if strings.HasPrefix(tagName, "h") {
			text := strings.TrimSpace(s.Text())
			if text != "" && (tagName == "h1" || tagName == "h2" || tagName == "h3") {
				currentSection = text
			}
			// Also check if heading contains a link
			s.Find("a").Each(func(j int, link *goquery.Selection) {
				if info := processLink(link, base, tagName, currentSection, seen); info != nil {
					links = append(links, *info)
				}
			})
			return
		}

		// Process standalone links
		if tagName == "a" {
			// Determine context from parent elements
			context := getParentContext(s)
			if info := processLink(s, base, context, currentSection, seen); info != nil {
				links = append(links, *info)
			}
		}
	})

	// Sort by score descending
	sortLinksByScore(links)

	return links, nil
}

// processLink extracts info from a link element
func processLink(s *goquery.Selection, base *url.URL, context, section string, seen map[string]bool) *LinkInfo {
	href, exists := s.Attr("href")
	if !exists || href == "" {
		return nil
	}

	// Skip anchors, javascript, and mailto
	if strings.HasPrefix(href, "#") || strings.HasPrefix(href, "javascript:") || strings.HasPrefix(href, "mailto:") {
		return nil
	}

	// Resolve relative URLs
	resolved := resolveURL(href, base)
	if resolved == "" {
		return nil
	}

	// Skip duplicates
	if seen[resolved] {
		return nil
	}
	seen[resolved] = true

	text := strings.TrimSpace(s.Text())
	if text == "" {
		// Try alt text from images
		if img := s.Find("img"); img.Length() > 0 {
			text, _ = img.Attr("alt")
		}
		// Try title attribute
		if text == "" {
			text, _ = s.Attr("title")
		}
	}

	// Calculate score
	score := calculateLinkScore(s, context)

	return &LinkInfo{
		Href:    resolved,
		Text:    text,
		Context: context,
		Score:   score,
		Section: section,
	}
}

// getParentContext determines the semantic context of a link
func getParentContext(s *goquery.Selection) string {
	// Check parents for context
	contexts := []string{"h1", "h2", "h3", "h4", "h5", "h6", "nav", "main", "article", "aside", "footer", "header", "li", "p"}

	for _, ctx := range contexts {
		if s.ParentsFiltered(ctx).Length() > 0 {
			return ctx
		}
	}

	return "body"
}

// calculateLinkScore calculates importance score for a link
func calculateLinkScore(s *goquery.Selection, context string) int {
	// Base scores by context
	baseScores := map[string]int{
		"h1":      100,
		"h2":      85,
		"h3":      70,
		"h4":      55,
		"h5":      55,
		"h6":      55,
		"main":    50,
		"article": 50,
		"p":       40,
		"li":      35,
		"nav":     30,
		"header":  25,
		"aside":   20,
		"footer":  10,
		"body":    30,
	}

	score := baseScores[context]
	if score == 0 {
		score = 30 // default
	}

	// Modifiers
	// Check if link or its parents have bold/strong
	if s.ParentsFiltered("strong, b").Length() > 0 || s.Find("strong, b").Length() > 0 {
		score += 15
	}

	// Check for emphasis
	if s.ParentsFiltered("em, i").Length() > 0 || s.Find("em, i").Length() > 0 {
		score += 10
	}

	// Check for button-like elements
	if role, _ := s.Attr("role"); role == "button" {
		score += 10
	}
	if class, _ := s.Attr("class"); strings.Contains(class, "btn") || strings.Contains(class, "button") {
		score += 10
	}

	// Has title attribute
	if title, _ := s.Attr("title"); title != "" {
		score += 5
	}

	return score
}

// resolveURL resolves a potentially relative URL against a base
func resolveURL(href string, base *url.URL) string {
	parsed, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(parsed)
	return resolved.String()
}

// sortLinksByScore sorts links by score in descending order
func sortLinksByScore(links []LinkInfo) {
	for i := 0; i < len(links)-1; i++ {
		for j := i + 1; j < len(links); j++ {
			if links[j].Score > links[i].Score {
				links[i], links[j] = links[j], links[i]
			}
		}
	}
}

// LLMRankRequest represents the input for LLM ranking
type LLMRankRequest struct {
	Context string         `json:"context"`
	Links   []LLMLinkInput `json:"links"`
}

// LLMLinkInput represents a link for LLM input
type LLMLinkInput struct {
	Index   int    `json:"index"`
	Href    string `json:"href"`
	Text    string `json:"text"`
	Section string `json:"section,omitempty"`
}

// LLMRankResponse represents the LLM's ranking response
type LLMRankResponse struct {
	Rankings []LLMRanking `json:"rankings"`
}

// LLMRanking represents a single link's relevance score
type LLMRanking struct {
	Index int `json:"index"`
	Score int `json:"score"`
}

// rankLinksWithLLM uses an LLM to rank links by relevance to the given context
func rankLinksWithLLM(links []LinkInfo, context string, timeout int) ([]LinkInfo, error) {
	if len(links) == 0 {
		return links, nil
	}

	// Load API configuration
	config := llmapi.GetAPIConfig()
	if err := config.Validate(); err != nil {
		return links, fmt.Errorf("API configuration error: %w", err)
	}

	// Create LLM client
	client := llmapi.NewLLMClientFromConfig(config)

	// Prepare links for LLM input (limit to avoid token limits)
	maxLinks := 100
	if len(links) > maxLinks {
		links = links[:maxLinks]
	}

	llmLinks := make([]LLMLinkInput, len(links))
	for i, link := range links {
		llmLinks[i] = LLMLinkInput{
			Index:   i,
			Href:    link.Href,
			Text:    link.Text,
			Section: link.Section,
		}
	}

	// Build the prompt
	systemPrompt := `You are a link relevance analyzer. Given a context and a list of links, score each link's relevance to the context on a scale of 0-100.

Rules:
1. Score 90-100: Directly related, primary resource for the context
2. Score 70-89: Closely related, useful secondary resource
3. Score 50-69: Somewhat related, tangentially useful
4. Score 30-49: Loosely related, might be useful
5. Score 0-29: Not relevant to the context

Respond ONLY with valid JSON in this exact format:
{"rankings":[{"index":0,"score":85},{"index":1,"score":45},...]}

Do not include any explanation or text outside the JSON.`

	linksJSON, err := json.Marshal(llmLinks)
	if err != nil {
		return links, fmt.Errorf("failed to marshal links: %w", err)
	}

	userPrompt := fmt.Sprintf("Context: %s\n\nLinks to rank:\n%s", context, string(linksJSON))

	// Truncate if too long
	if len(userPrompt) > 25000 {
		userPrompt = userPrompt[:25000] + "\n...(truncated)"
	}

	// Call the LLM
	response, err := client.CompleteWithSystem(systemPrompt, userPrompt, time.Duration(timeout)*time.Second)
	if err != nil {
		return links, fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse the response
	rankings, err := parseLLMRankings(response, len(links))
	if err != nil {
		return links, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Apply scores to links
	for _, ranking := range rankings {
		if ranking.Index >= 0 && ranking.Index < len(links) {
			links[ranking.Index].Score = ranking.Score
		}
	}

	// Re-sort by new scores
	sortLinksByScore(links)

	return links, nil
}

// parseLLMRankings parses the LLM response into rankings
func parseLLMRankings(response string, linkCount int) ([]LLMRanking, error) {
	// Clean up response - find JSON object
	response = strings.TrimSpace(response)

	// Find the JSON object boundaries
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no valid JSON found in response")
	}
	response = response[start : end+1]

	var result LLMRankResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	// Validate rankings
	for i, r := range result.Rankings {
		if r.Score < 0 {
			result.Rankings[i].Score = 0
		} else if r.Score > 100 {
			result.Rankings[i].Score = 100
		}
	}

	return result.Rankings, nil
}

func outputExtractLinksResult(cmd *cobra.Command, result ExtractLinksResult) error {
	var finalResult ExtractLinksResult

	if extractLinksMinimal {
		// Convert to minimal format
		minLinks := make([]LinkInfo, len(result.Links))
		for i, link := range result.Links {
			score := link.Score
			minLinks[i] = LinkInfo{
				H:   link.Href,
				T:   link.Text,
				C:   link.Context,
				S:   &score,
				Sec: link.Section,
			}
		}
		total := result.Total
		finalResult = ExtractLinksResult{
			U: result.URL,
			L: minLinks,
			N: &total,
			E: result.Error,
		}
	} else {
		finalResult = result
	}

	formatter := output.New(extractLinksJSON, extractLinksMinimal, cmd.OutOrStdout())
	return formatter.Print(finalResult, func(w io.Writer, data interface{}) {
		r := data.(ExtractLinksResult)
		if r.Error != "" {
			fmt.Fprintf(w, "ERROR: %s\n", r.Error)
			return
		}

		fmt.Fprintf(w, "URL: %s\n", result.URL)
		fmt.Fprintf(w, "Total Links: %d\n\n", result.Total)

		for _, link := range result.Links {
			fmt.Fprintf(w, "[%d] %s\n", link.Score, link.Text)
			fmt.Fprintf(w, "    %s\n", link.Href)
			if link.Section != "" {
				fmt.Fprintf(w, "    Section: %s | Context: %s\n", link.Section, link.Context)
			} else {
				fmt.Fprintf(w, "    Context: %s\n", link.Context)
			}
			fmt.Fprintln(w)
		}
	})
}

func init() {
	RootCmd.AddCommand(newExtractLinksCmd())
}
