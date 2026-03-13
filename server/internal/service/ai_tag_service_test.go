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
		{name: "clip classify", input: "clip_classify", output: "ai"},
		{name: "scene classify", input: "clip_scene_classify", output: "ai"},
		{name: "bioclip classify", input: "bioclip_classify", output: "ai"},
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
		"clip_classify",
		"clip_scene_classify",
		"bioclip_classify",
		"ai",
		"user",
		"",
		"user",
	})

	want := []string{"ai", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeAssetTagSources() = %#v, want %#v", got, want)
	}
}
