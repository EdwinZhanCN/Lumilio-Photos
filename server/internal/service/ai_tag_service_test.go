package service

import (
	"reflect"
	"testing"
)

func TestNormalizeAssetTagSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		output string
	}{
		{name: "empty", input: "", output: ""},
		{name: "system", input: "system", output: "system"},
		{name: "user", input: "user", output: "user"},
		{name: "ai", input: "ai", output: "ai"},
		{name: "bioclip classify", input: "bioclip_classify", output: "bioclip_classify"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeAssetTagSource(tt.input); got != tt.output {
				t.Fatalf("normalizeAssetTagSource(%q) = %q, want %q", tt.input, got, tt.output)
			}
		})
	}
}

func TestNormalizeAssetTagSourcesDedupesNormalizedValues(t *testing.T) {
	t.Parallel()

	got := normalizeAssetTagSources([]string{
		"bioclip_classify",
		"ai",
		"user",
		"",
		"user",
	})

	want := []string{"bioclip_classify", "ai", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAssetTagSources() = %#v, want %#v", got, want)
	}
}
