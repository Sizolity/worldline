// Package textutil holds small, domain-neutral helpers for cleaning up raw
// LLM text output. It is a leaf package with no world/* dependencies so any
// layer (director, runtime, …) can reuse it without reaching across feature
// packages.
package textutil

import "strings"

// StripMarkdownFences removes ```json ... ``` or ``` ... ``` wrappers that
// LLMs commonly add despite being told not to.
func StripMarkdownFences(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	// Remove opening fence line
	idx := strings.Index(trimmed, "\n")
	if idx < 0 {
		return trimmed
	}
	inner := trimmed[idx+1:]
	// Remove closing fence
	if last := strings.LastIndex(inner, "```"); last >= 0 {
		inner = inner[:last]
	}
	return strings.TrimSpace(inner)
}
