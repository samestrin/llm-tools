package commands

// coherence.go implements the optional `td-validate --coherence` check, which
// flags TD rows whose FIX cell reads as incoherent with its PROBLEM cell (the
// signature of a copy-pasted-from-an-unrelated-finding fix). The check is
// advisory: it ranks rows by an ensemble of graceful-degrading signals and
// never changes the file/symbol validation result or the process exit code.

// --- pure helpers (RED stubs; implemented in GREEN) ---

// cosine returns the cosine similarity of two equal-length vectors, or 0 when
// the lengths differ or either vector has zero magnitude.
func cosine(a, b []float32) float64 { return 0 }

// stripCoherenceBoilerplate removes the near-identical deferral/clarification
// annotations that appear across many TD rows, so they do not inflate
// cross-row textual similarity.
func stripCoherenceBoilerplate(s string) string { return s }

// extractCodeIdentifiers returns the set of code-shaped tokens in s (backtick
// spans, camelCase, PascalCase, snake_case, dotted/path segments), lowercased.
func extractCodeIdentifiers(s string) map[string]struct{} { return nil }

// isDeferralPunt reports whether a FIX cell defers/hands off rather than
// prescribing a technical remedy (the dominant false-positive class in the
// low-similarity tail).
func isDeferralPunt(fix string) bool { return false }
