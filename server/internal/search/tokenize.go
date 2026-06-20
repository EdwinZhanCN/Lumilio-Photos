package search

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenizeForSearch converts raw text into a search-optimized string using CJK
// bigram tokenization. CJK character runs are split into overlapping 2-character
// pairs; non-CJK runs are kept as whole words. Both the write path (OCR ingest)
// and the query path must use this function so that tokens align.
//
// Example: "听说你还在找白样 hello" → "听说 说你 你还 还在 在找 找白 白样 hello"
func TokenizeForSearch(text string) string {
	tokens := tokenize(text)
	return strings.Join(tokens, " ")
}

func tokenize(text string) []string {
	var tokens []string
	runes := []rune(text)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if unicode.IsSpace(r) || r <= 32 {
			i++
			continue
		}

		if isCJK(r) {
			start := i
			for i < len(runes) && isCJK(runes[i]) {
				i++
			}
			cjkRun := runes[start:i]
			if len(cjkRun) == 1 {
				tokens = append(tokens, string(cjkRun))
			} else {
				for k := 0; k < len(cjkRun)-1; k++ {
					tokens = append(tokens, string(cjkRun[k:k+2]))
				}
			}
		} else {
			start := i
			for i < len(runes) && runes[i] > 32 && !unicode.IsSpace(runes[i]) && !isCJK(runes[i]) {
				i++
			}
			tokens = append(tokens, string(runes[start:i]))
		}
	}
	return tokens
}

func isCJK(r rune) bool {
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0xAC00 && r <= 0xD7AF) || // Hangul Syllables
		(r >= 0x3040 && r <= 0x309F) || // Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0x3400 && r <= 0x4DBF) // CJK Unified Ideographs Extension A
}

// TokenizeQuery tokenizes a user query for trigram search. Same logic as
// TokenizeForSearch — kept as a separate entry point for clarity at call sites.
func TokenizeQuery(query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return ""
	}
	if !containsCJK(q) {
		return q
	}
	return TokenizeForSearch(q)
}

func containsCJK(s string) bool {
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if isCJK(r) {
			return true
		}
		i += size
	}
	return false
}
