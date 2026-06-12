package ref

import (
	"strings"
	"unicode"
)

// Default per-field budgets for user-content text surfaced to the LLM.
const (
	MaxFacetValueLen = 40 // describe facet values (place labels, names, camera models)
	MaxPeekFieldLen  = 80 // peek line fields (filenames, place labels)
)

// SanitizeUserText is the single exit point for text that originates from
// user content (OCR fragments, place labels, person names, filenames) before
// it is surfaced to the LLM (INV-7). It strips control and zero-width runes,
// collapses whitespace and truncates to maxLen runes. Sanitized values must
// only ever be emitted as structured data values, never spliced into
// instruction positions of a prompt.
func SanitizeUserText(s string, maxLen int) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true // suppress leading whitespace
	for _, r := range s {
		// Whitespace first: \t and \n are also control characters and must
		// collapse to a space rather than vanish.
		if unicode.IsSpace(r) {
			if !lastSpace {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		if unicode.IsControl(r) || isZeroWidth(r) {
			continue
		}
		b.WriteRune(r)
		lastSpace = false
	}
	out := strings.TrimRight(b.String(), " ")
	if maxLen > 0 {
		runes := []rune(out)
		if len(runes) > maxLen {
			out = string(runes[:maxLen]) + "…"
		}
	}
	return out
}

func isZeroWidth(r rune) bool {
	switch r {
	case '\u200b', '\u200c', '\u200d', '\u2060', '\ufeff':
		return true
	}
	return false
}
