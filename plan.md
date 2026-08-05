Ready to code?

 Here is Claude's plan:
╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌╌
 Rebuild the llm-support fetch command

 Context

 A proxy- and FlareSolverr-aware fetch command was previously intended for llm-support but
 was never committed (verified: no flaresolverr/proxy/fetch matches across the working tree,
 all branches, or the full git log). What does exist is the config side: the global
 ~/.claude/settings.json already exports the env vars the command was meant to consume —
 FETCH_FLARESOLVERR_URL=http://orchestrator.lan:8191/ and FETCH_PROXY_URL=http://…@p.webshare.io:80.
 Nothing reads them anywhere under ~/Documents/GitHub.

 The proven implementation lives in the Python repo brave-serp
 (src/brave_scrape/fetcher.py). This plan ports its minimal faithful core into Go as a new
 llm-support fetch subcommand: fetch a URL through the rotating residential proxy, fall back to
 FlareSolverr on block codes, render JavaScript pages on demand, and emit the content as
 markdown (default), raw HTML, or plain text.

 Outcome: llm-support fetch <url> rebuilt, documented, MCP-exposed, ≥80% covered, merged to main.

 Decisions locked in (user-confirmed)

 - Scope: minimal faithful core only — no Playwright/Wayback/PDF/circuit-breaker/weighted-proxy.
 - Markdown: add github.com/JohannesKaufmann/html-to-markdown (real markdown: links, bold, headings).
 - FlareSolverr proxy: FlareSolverr fetches directly (its own browser/TLS fingerprint), mirroring brave-serp; the proxy applies only to the direct GET.
 - JavaScript pages: supported via FlareSolverr's headless Chrome (it runs JS). A --render flag forces the FlareSolverr path — no new browser dependency.

 Design

 New command llm-support fetch <url>.

 Flags
 - -f, --format — markdown (default) | html | text
 - --render (alias --js) — force the FlareSolverr path (executes JavaScript, returns rendered DOM)
 - -t, --timeout — seconds, default 30
 - -o, --output — write content to a file instead of stdout (the "download" behavior)
 - --json / --min — global flags; JSON wraps metadata {url, status, source, format, content}

 Flow
 1. Validate the URL (reuse the isURL pattern from extract_relevant.go:173).
 2. If --render: go straight to FlareSolverr.
 3. Else direct GET: http.Client with Transport.Proxy = http.ProxyURL(FETCH_PROXY_URL) when set
 (rotation is server-side at the webshare gateway), Chrome-like User-Agent, --timeout.
 4. On result:
   - 200 → use body.
   - 403 / 429 / 503 → fall back to FlareSolverr if FETCH_FLARESOLVERR_URL set (one light retry on 429/503 first).
   - other non-2xx → error.
 5. FlareSolverr: normalize URL (append /v1 if absent — config value lacks it), POST
 {"cmd":"request.get","url":<url>,"maxTimeout":60000}, read HTML from solution.response.
 6. Convert per --format: html = raw; markdown = html-to-markdown lib; text = reuse
 htmlToText (extract_relevant.go:238, same package — call directly).
 7. Write to stdout or --output file.

 Reuse
 - htmlToText(io.Reader) (string, error) — internal/support/commands/extract_relevant.go:238 (text format).
 - isURL(string) bool — extract_relevant.go:173.
 - output.New(...).Print(...) — pkg/output/formatter.go:64 (JSON/min/text dispatch).
 - Cobra command shape — internal/support/commands/hash.go as the template.

 Files

 Create
 - internal/support/commands/fetch.go — command + small testable helpers:
 runFetch, buildHTTPClient(timeout), fetchDirect, fetchViaFlareSolverr,
 normalizeFlareSolverrURL, convertContent(html, format).
 - internal/support/commands/fetch_test.go — tests (see Verification).

 Modify
 - internal/support/mcpserver/tools.go — add llm_support_fetch tool definition (schema: url required; format, render, timeout optional).
 - internal/support/mcpserver/tools_test.go:12 — 73 → 74.
 - tests/mcp_integration/schema_test.go:15 — 73 → 74.
 - go.mod / go.sum — add html-to-markdown (go mod tidy).
 - README.md (and any docs/ command listing) — document fetch: usage, three formats, --render, and the two FETCH_* env vars.

 TDD execution (commit tests before implementation where possible)

 Per-task: Complex → Red→Green→Adversarial(auto-fix Med/High)→Refactor; Simple → Red→Green→Refactor. Keep ≥80% coverage throughout; commit frequently.

 1. Branch + dependency (simple) — create feat/llm-support-fetch; go get html-to-markdown; go mod tidy; build sanity.
 2. Content conversion + helpers (simple) — convertContent, normalizeFlareSolverrURL, URL validation. Table-driven tests first.
 3. Proxy HTTP client + direct fetch (complex) — buildHTTPClient (asserts Transport.Proxy resolves FETCH_PROXY_URL), fetchDirect against httptest.Server for 200/403/429/503 paths.
 4. FlareSolverr fallback + --render (complex) — second httptest.Server returning FlareSolverr JSON at /v1; test fallback trigger on block codes, forced render, and JS-rendered DOM passthrough.
 5. Command wiring (complex) — cobra flags, stdout vs --output, --json/--min metadata; full cmd.Execute() tests incl. missing-arg error.
 6. MCP registration (simple) — add tool def; bump both counts to 74; run MCP tests.
 7. Documentation (simple) — README + docs entry.

 Verification

 - Unit/integration: go test ./internal/support/... ./tests/... -cover — every new helper covered; package ≥80%.
   - HTTP via httptest.Server (200 HTML, 403→FlareSolverr success, --render forces FlareSolverr, each format, timeout, missing URL).
   - Env handled with t.Setenv (FETCH_PROXY_URL, FETCH_FLARESOLVERR_URL → test server URL).
 - MCP counts: go test ./internal/support/mcpserver/... ./tests/mcp_integration/... green at 74.
 - Build all binaries per CLAUDE.md (build/ targets).
 - Live smoke (manual, with real env): build/llm-support fetch https://example.com (markdown),
 --format html, --format text, and --render <a JS-heavy URL> confirming rendered content.

 Cumulative adversarial review (after all tasks)

 Hostile end-to-end pass: secret/credential leakage (proxy URL contains creds — never log it, never echo in errors/JSON), error handling on FlareSolverr non-ok status, malformed/empty solution.response, non-HTML content types, redirect handling, --output
 path safety, timeout correctness, and the FlareSolverr /v1 normalization edge cases. Fix every Med/High finding. Ask "release-ready?"; fix until yes.

 Ship

 Create PR feat/llm-support-fetch → main, merge, delete the branch. (No old feature branch exists to remove — the lost command was never committed — so "delete the old branch" applies to the new feature branch post-merge.)
╌
