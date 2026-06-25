package fetch

import (
	"net/http"
	"net/url"
	"strings"
	"time"
)

// proxyClient builds an http.Client that routes every request through proxyURL.
// IP rotation is the gateway's responsibility (one URL, fresh IP per request),
// so nothing here tracks state. A proxy URL without a scheme is treated as http.
func proxyClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	parsed, err := url.Parse(normalizeProxyURL(proxyURL))
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: http.ProxyURL(parsed)}
	return &http.Client{Timeout: timeout, Transport: transport}, nil
}

// normalizeProxyURL defaults a schemeless proxy (e.g. user:pass@host:port) to http.
func normalizeProxyURL(raw string) string {
	if raw == "" {
		return raw
	}
	if !strings.Contains(raw, "://") {
		return "http://" + raw
	}
	return raw
}
