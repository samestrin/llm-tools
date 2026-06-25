package fetch

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

// Output formats for Render.
const (
	FormatRaw      = "raw"
	FormatText     = "text"
	FormatMarkdown = "markdown"
)

var multiBlankLine = regexp.MustCompile(`\n[ \t]*\n([ \t]*\n)+`)

// Render converts body to the requested format. raw (and "html") is a
// passthrough. text/markdown only transform when the content looks like HTML —
// non-HTML payloads (JSON, images, ...) are returned unchanged so the format
// flag never corrupts a non-HTML download.
func Render(body []byte, contentType, format string) ([]byte, error) {
	switch format {
	case "", FormatRaw, "html":
		return body, nil
	}
	if !isHTML(contentType, body) {
		return body, nil
	}
	switch format {
	case FormatText:
		return htmlToText(body)
	case FormatMarkdown:
		return htmlToMarkdown(body)
	default:
		return nil, fmt.Errorf("unknown format %q (want raw, text, or markdown)", format)
	}
}

// isHTML decides whether body should be treated as HTML, preferring the
// declared content-type and falling back to a cheap body sniff.
func isHTML(contentType string, body []byte) bool {
	if contentType != "" {
		return strings.Contains(strings.ToLower(contentType), "html")
	}
	head := body
	if len(head) > 512 {
		head = head[:512]
	}
	lower := strings.ToLower(string(head))
	return strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html")
}

func htmlToText(body []byte) ([]byte, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	doc.Find("script, style, noscript, template, svg").Remove()

	sel := doc.Find("body")
	if sel.Length() == 0 {
		sel = doc.Selection
	}
	return []byte(collapseBlankLines(sel.Text())), nil
}

func htmlToMarkdown(body []byte) ([]byte, error) {
	conv := md.NewConverter("", true, nil)
	out, err := conv.ConvertString(string(body))
	if err != nil {
		return nil, fmt.Errorf("convert markdown: %w", err)
	}
	return []byte(collapseBlankLines(out)), nil
}

// collapseBlankLines trims trailing spaces per line, collapses 3+ consecutive
// blank lines down to one, and trims surrounding whitespace.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimRight(ln, " \t")
	}
	joined := strings.Join(lines, "\n")
	joined = multiBlankLine.ReplaceAllString(joined, "\n\n")
	return strings.TrimSpace(joined) + "\n"
}
