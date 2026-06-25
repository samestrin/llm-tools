package fetch

import "testing"

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("FETCH_FLARESOLVERR_URL", "")
	t.Setenv("FETCH_PROXY_URL", "")
	t.Setenv("FETCH_TIMEOUT", "")
	t.Setenv("FETCH_MAX_RETRIES", "")
	t.Setenv("FETCH_USER_AGENT", "")

	cfg := LoadConfig()
	if cfg.FlareSolverrURL != "" {
		t.Errorf("FlareSolverrURL = %q, want empty", cfg.FlareSolverrURL)
	}
	if cfg.ProxyURL != "" {
		t.Errorf("ProxyURL = %q, want empty", cfg.ProxyURL)
	}
	if cfg.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %d, want %d", cfg.Timeout, DefaultTimeout)
	}
	if cfg.MaxRetries != DefaultMaxRetries {
		t.Errorf("MaxRetries = %d, want %d", cfg.MaxRetries, DefaultMaxRetries)
	}
	if cfg.UserAgent != DefaultUserAgent {
		t.Errorf("UserAgent = %q, want %q", cfg.UserAgent, DefaultUserAgent)
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("FETCH_FLARESOLVERR_URL", "http://localhost:8191")
	t.Setenv("FETCH_PROXY_URL", "http://user:pass@proxy:80")
	t.Setenv("FETCH_TIMEOUT", "45")
	t.Setenv("FETCH_MAX_RETRIES", "5")
	t.Setenv("FETCH_USER_AGENT", "custom-agent")

	cfg := LoadConfig()
	if cfg.FlareSolverrURL != "http://localhost:8191" {
		t.Errorf("FlareSolverrURL = %q", cfg.FlareSolverrURL)
	}
	if cfg.ProxyURL != "http://user:pass@proxy:80" {
		t.Errorf("ProxyURL = %q", cfg.ProxyURL)
	}
	if cfg.Timeout != 45 {
		t.Errorf("Timeout = %d, want 45", cfg.Timeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", cfg.MaxRetries)
	}
	if cfg.UserAgent != "custom-agent" {
		t.Errorf("UserAgent = %q, want custom-agent", cfg.UserAgent)
	}
}

// getEnvInt falls back to the default on non-numeric and non-positive values.
func TestGetEnvIntInvalid(t *testing.T) {
	cases := []struct {
		name string
		val  string
	}{
		{"non-numeric", "abc"},
		{"zero", "0"},
		{"negative", "-3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FETCH_TIMEOUT", tc.val)
			if got := getEnvInt("FETCH_TIMEOUT", DefaultTimeout); got != DefaultTimeout {
				t.Errorf("getEnvInt(%q) = %d, want default %d", tc.val, got, DefaultTimeout)
			}
		})
	}
}
