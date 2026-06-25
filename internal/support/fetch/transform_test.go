package fetch

import (
	"strings"
	"testing"
)

const sampleHTML = `<!doctype html><html><head><title>T</title>
<style>.x{color:red}</style><script>var a=1;</script></head>
<body><h1>Title</h1><p>Hello <a href="https://x.test">link</a> and <code>code</code>.</p>
<ul><li>one</li><li>two</li></ul></body></html>`

func TestRenderRawPassthrough(t *testing.T) {
	out, err := Render([]byte(sampleHTML), "text/html", FormatRaw)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != sampleHTML {
		t.Error("raw should be unchanged")
	}
}

func TestRenderText(t *testing.T) {
	out, err := Render([]byte(sampleHTML), "text/html", FormatText)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "Title") || !strings.Contains(s, "Hello") {
		t.Errorf("missing content: %q", s)
	}
	if strings.Contains(s, "var a=1") || strings.Contains(s, "color:red") {
		t.Errorf("script/style not stripped: %q", s)
	}
	if len(out) >= len(sampleHTML) {
		t.Errorf("text (%d) should be smaller than html (%d)", len(out), len(sampleHTML))
	}
}

func TestRenderMarkdown(t *testing.T) {
	out, err := Render([]byte(sampleHTML), "text/html", FormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "# Title") {
		t.Errorf("heading not converted: %q", s)
	}
	if !strings.Contains(s, "[link](https://x.test)") {
		t.Errorf("link not preserved: %q", s)
	}
	if !strings.Contains(s, "`code`") {
		t.Errorf("code not preserved: %q", s)
	}
}

func TestRenderNonHTMLPassthrough(t *testing.T) {
	jsonBody := []byte(`{"a":1,"b":2}`)
	out, err := Render(jsonBody, "application/json", FormatMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != string(jsonBody) {
		t.Errorf("non-HTML should pass through unchanged, got %q", out)
	}
}

func TestRenderUnknownFormat(t *testing.T) {
	if _, err := Render([]byte(sampleHTML), "text/html", "pdf"); err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestIsHTMLSniff(t *testing.T) {
	if !isHTML("", []byte("<!DOCTYPE html><html>")) {
		t.Error("should sniff doctype as HTML")
	}
	if isHTML("application/json", []byte("{}")) {
		t.Error("json content-type is not HTML")
	}
}
