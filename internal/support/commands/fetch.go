package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/samestrin/llm-tools/internal/support/fetch"
	"github.com/samestrin/llm-tools/pkg/output"
)

var (
	fetchOutput       string
	fetchFormat       string
	fetchVia          string
	fetchFlareSolverr string
	fetchProxy        string
	fetchTimeout      int
	fetchRetries      int
	fetchNoFallback   bool
	fetchUserAgent    string
	fetchJSON         bool
	fetchMinimal      bool
)

// FetchResult is the serializable outcome of a fetch.
type FetchResult struct {
	URL      string `json:"url,omitempty"`
	FinalURL string `json:"final_url,omitempty"`
	Status   int    `json:"status,omitempty"`
	Tier     string `json:"tier,omitempty"`
	Format   string `json:"format,omitempty"`
	Bytes    int    `json:"bytes,omitempty"`
	Output   string `json:"output,omitempty"`
	Body     string `json:"body,omitempty"`
	Error    string `json:"error,omitempty"`
}

func newFetchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Download a URL with FlareSolverr and rotating-proxy fallback",
		Long: `Download a URL using a smart fallback ladder.

The direct HTTP request is tried first. On an IP-level block (403/429/5xx or a
connection failure) the request is retried through a rotating HTTP proxy. On a
Cloudflare/JS challenge page the request is sent to FlareSolverr, which renders
it in a headless browser and solves the challenge.

Configuration is read from environment variables (flags override them):

  FETCH_FLARESOLVERR_URL   FlareSolverr endpoint (e.g. http://localhost:8191)
  FETCH_PROXY_URL          Rotating proxy gateway (e.g. http://user:pass@host:port)
  FETCH_TIMEOUT            Per-attempt timeout in seconds (default 30)
  FETCH_MAX_RETRIES        Retries within a tier (default 2)
  FETCH_USER_AGENT         Override the request User-Agent

With neither FlareSolverr nor a proxy configured, this is a plain HTTP GET.

By default the response body is written to stdout (curl-like). Use --output to
save it to a file instead. The result reports which tier succeeded.

OUTPUT FORMAT (--format, HTML only; non-HTML passes through unchanged):
  raw       Body exactly as received (default)
  text      Tags stripped to readable text (smallest)
  markdown  Structure-preserving Markdown — best for LLM consumption

ROUTING (--via):
  auto          Full fallback ladder (default)
  direct        Direct tier only (same as --no-fallback)
  proxy         Force the rotating proxy
  flaresolverr  Force the FlareSolverr renderer

Examples:
  llm-support fetch https://example.com
  llm-support fetch https://example.com --format markdown
  llm-support fetch https://example.com --format text --output page.txt
  llm-support fetch https://example.com --json
  llm-support fetch https://example.com --via proxy
  llm-support fetch https://example.com --via flaresolverr`,
		Args: cobra.ExactArgs(1),
		RunE: runFetch,
	}

	cmd.Flags().StringVar(&fetchOutput, "output", "", "Write the body to this file instead of stdout")
	cmd.Flags().StringVar(&fetchFormat, "format", fetch.FormatRaw, "Output format: raw, text, or markdown (HTML only)")
	cmd.Flags().StringVar(&fetchFlareSolverr, "flaresolverr", "", "FlareSolverr endpoint (overrides FETCH_FLARESOLVERR_URL)")
	cmd.Flags().StringVar(&fetchProxy, "proxy", "", "Rotating proxy URL (overrides FETCH_PROXY_URL)")
	cmd.Flags().IntVar(&fetchTimeout, "timeout", fetch.DefaultTimeout, "Per-attempt timeout in seconds")
	cmd.Flags().IntVar(&fetchRetries, "retries", fetch.DefaultMaxRetries, "Retries within a tier")
	cmd.Flags().StringVar(&fetchVia, "via", fetch.ViaAuto, "Routing: auto, direct, proxy, or flaresolverr")
	cmd.Flags().BoolVar(&fetchNoFallback, "no-fallback", false, "Direct fetch only (alias for --via direct)")
	cmd.Flags().StringVar(&fetchUserAgent, "user-agent", "", "Override the request User-Agent")
	cmd.Flags().BoolVar(&fetchJSON, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&fetchMinimal, "min", false, "Output in minimal/token-optimized format")

	return cmd
}

func runFetch(cmd *cobra.Command, args []string) error {
	rawURL := args[0]
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	// Env config, with explicit flags taking precedence.
	cfg := fetch.LoadConfig()
	if fetchFlareSolverr != "" {
		cfg.FlareSolverrURL = fetchFlareSolverr
	}
	if fetchProxy != "" {
		cfg.ProxyURL = fetchProxy
	}
	if cmd.Flags().Changed("timeout") {
		cfg.Timeout = fetchTimeout
	}
	if cmd.Flags().Changed("retries") {
		cfg.MaxRetries = fetchRetries
	}
	if fetchUserAgent != "" {
		cfg.UserAgent = fetchUserAgent
	}

	// Resolve routing: --no-fallback is an alias for --via direct.
	via := fetchVia
	if fetchNoFallback {
		via = fetch.ViaDirect
	}
	switch via {
	case fetch.ViaAuto, fetch.ViaDirect, fetch.ViaProxy, fetch.ViaFlareSolverr:
	default:
		return fmt.Errorf("invalid --via %q (want auto, direct, proxy, or flaresolverr)", via)
	}

	res := fetch.New(cfg).Fetch(rawURL, via)

	out := FetchResult{
		URL:      res.URL,
		FinalURL: res.FinalURL,
		Status:   res.Status,
		Tier:     string(res.Tier),
		Format:   fetchFormat,
		Bytes:    res.Bytes,
	}
	if res.Err != nil {
		out.Error = res.Err.Error()
	}

	// Apply the output transform (raw is a passthrough; non-HTML is untouched).
	var rendered []byte
	if res.Err == nil {
		var rerr error
		rendered, rerr = fetch.Render(res.Body, res.ContentType, fetchFormat)
		if rerr != nil {
			return rerr
		}
		out.Bytes = len(rendered)
	}

	// Write body to a file when requested.
	if fetchOutput != "" && res.Err == nil {
		if err := os.WriteFile(fetchOutput, rendered, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		out.Output = fetchOutput
	}

	if fetchJSON {
		if fetchOutput == "" && res.Err == nil {
			out.Body = string(rendered)
		}
		formatter := output.New(true, fetchMinimal, cmd.OutOrStdout())
		return formatter.Print(out, nil)
	}

	// Text mode.
	w := cmd.OutOrStdout()
	if res.Err != nil {
		fmt.Fprintf(w, "ERROR: %s\n", res.Err.Error())
		fmt.Fprintf(w, "URL: %s | tier: %s\n", res.URL, res.Tier)
		return nil
	}
	if fetchOutput != "" {
		fmt.Fprintf(w, "Saved %d bytes [HTTP %d via %s, %s] -> %s\n", out.Bytes, res.Status, res.Tier, fetchFormat, fetchOutput)
		return nil
	}
	w.Write(rendered)
	return nil
}

func init() {
	RootCmd.AddCommand(newFetchCmd())
}
