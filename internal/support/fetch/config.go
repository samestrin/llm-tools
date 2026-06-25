// Package fetch downloads URLs with a smart fallback ladder:
// direct HTTP first, then a rotating HTTP proxy on IP-level blocks, then
// FlareSolverr on Cloudflare/JS challenge pages. All configuration is read
// from environment variables (mirroring pkg/llmapi/config.go), so values can
// be supplied through the MCP server's env block.
package fetch

import (
	"os"
	"strconv"
	"strings"
)

// Defaults applied when the corresponding env var / flag is unset.
const (
	DefaultTimeout    = 30 // seconds, per attempt
	DefaultMaxRetries = 2  // retries within a single tier
	DefaultUserAgent  = "llm-support/1.0 (fetch)"
)

// Config holds fetch configuration. Empty FlareSolverrURL / ProxyURL disable
// that tier — with neither set, Fetch is a plain HTTP GET.
type Config struct {
	FlareSolverrURL string
	ProxyURL        string
	Timeout         int
	MaxRetries      int
	UserAgent       string
	// FlareSolverrProxyURL is a proxy used only for the FlareSolverr render. Set
	// it to a no-auth relay: Chrome cannot authenticate proxies, so FlareSolverr's
	// authenticated-proxy path is flaky (ERR_NO_SUPPORTED_PROXIES). When empty the
	// render falls back to ProxyURL.
	FlareSolverrProxyURL string
	// FlareSolverrNoProxy keeps the FlareSolverr render direct even when a proxy
	// is configured (saves metered proxy bandwidth on sites that don't block it).
	FlareSolverrNoProxy bool
}

// LoadConfig reads configuration from environment variables.
//
//	FETCH_FLARESOLVERR_URL  FlareSolverr endpoint, e.g. http://localhost:8191
//	FETCH_PROXY_URL         Rotating proxy gateway, e.g. http://user:pass@host:port
//	FETCH_TIMEOUT           Per-attempt timeout in seconds (default 30)
//	FETCH_MAX_RETRIES       Retries within a tier (default 2)
//	FETCH_USER_AGENT        Override the request User-Agent
//	FETCH_FLARESOLVERR_PROXY_URL  Proxy for the FlareSolverr render (no-auth relay)
//	FETCH_FLARESOLVERR_NO_PROXY   Keep the FlareSolverr render direct (1/true/yes)
func LoadConfig() *Config {
	return &Config{
		FlareSolverrURL:      os.Getenv("FETCH_FLARESOLVERR_URL"),
		ProxyURL:             os.Getenv("FETCH_PROXY_URL"),
		Timeout:              getEnvInt("FETCH_TIMEOUT", DefaultTimeout),
		MaxRetries:           getEnvInt("FETCH_MAX_RETRIES", DefaultMaxRetries),
		UserAgent:            getEnvOrDefault("FETCH_USER_AGENT", DefaultUserAgent),
		FlareSolverrProxyURL: os.Getenv("FETCH_FLARESOLVERR_PROXY_URL"),
		FlareSolverrNoProxy:  getEnvBool("FETCH_FLARESOLVERR_NO_PROXY"),
	}
}

func getEnvBool(key string) bool {
	switch strings.ToLower(os.Getenv(key)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return n
		}
	}
	return defaultValue
}
