package fetch

import (
	"strings"
	"testing"
)

// A malformed proxy URL must never leak its embedded credentials through the
// surfaced error (url.Parse echoes the raw URL, which carries user:pass).
func TestProxyErrorDoesNotLeakCredentials(t *testing.T) {
	const secret = "s3cr3t-p4ss"
	// A control character in the password makes url.Parse fail, exercising the
	// error path that previously wrapped the raw (credential-bearing) URL.
	badProxy := "http://user:" + secret + "\x7f@proxy.example.com:80"

	cfg := testConfig()
	cfg.ProxyURL = badProxy
	res := New(cfg).Fetch("https://target/", ViaProxy)

	if res.Err == nil {
		t.Fatal("expected an error for a malformed proxy URL")
	}
	msg := res.Err.Error()
	if strings.Contains(msg, secret) {
		t.Errorf("error leaked proxy credentials: %q", msg)
	}
	if strings.Contains(msg, "user:") || strings.Contains(msg, "proxy.example.com") {
		t.Errorf("error leaked proxy URL contents: %q", msg)
	}
}
