package search

import (
	"testing"
)

func TestTokenizeForSearch(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "pure CJK bigrams",
			input: "听说你还在找你的白样",
			want:  "听说 说你 你还 还在 在找 找你 你的 的白 白样",
		},
		{
			name:  "single CJK char",
			input: "好",
			want:  "好",
		},
		{
			name:  "two CJK chars",
			input: "白样",
			want:  "白样",
		},
		{
			name:  "mixed CJK and Latin",
			input: "hello你好世界world",
			want:  "hello 你好 好世 世界 world",
		},
		{
			name:  "pure Latin",
			input: "hello world test",
			want:  "hello world test",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  "",
		},
		{
			name:  "CJK with spaces",
			input: "你好 世界",
			want:  "你好 世界",
		},
		{
			name:  "Japanese hiragana",
			input: "こんにちは",
			want:  "こん んに にち ちは",
		},
		{
			name:  "Korean hangul",
			input: "안녕하세요",
			want:  "안녕 녕하 하세 세요",
		},
		{
			name:  "numbers and CJK",
			input: "2025年白样测试",
			want:  "2025 年白 白样 样测 测试",
		},
		{
			name:  "punctuation between CJK",
			input: "你好，世界！",
			want:  "你好 ， 世界 ！",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenizeForSearch(tt.input)
			if got != tt.want {
				t.Errorf("TokenizeForSearch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTokenizeQuery(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "CJK query is tokenized",
			input: "白样",
			want:  "白样",
		},
		{
			name:  "Latin query passed through",
			input: "hello world",
			want:  "hello world",
		},
		{
			name:  "empty query",
			input: "",
			want:  "",
		},
		{
			name:  "mixed query",
			input: "test白样",
			want:  "test 白样",
		},
		{
			name:  "whitespace trimmed",
			input: "  白样  ",
			want:  "白样",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TokenizeQuery(tt.input)
			if got != tt.want {
				t.Errorf("TokenizeQuery(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
