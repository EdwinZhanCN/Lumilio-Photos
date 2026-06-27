package core

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

// newTestRegistry builds an isolated registry (not the process singleton) with
// a no-op factory registered for every tool named across all mode sets plus a
// couple of free-mode-only tools, so mode filtering can be asserted in
// isolation.
func newTestRegistry() *ToolRegistry {
	r := &ToolRegistry{
		factories: make(map[string]ToolFactory),
		infos:     make(map[string]*schema.ToolInfo),
	}
	names := map[string]bool{
		// free-mode-only tools (in no mode set) to prove they get filtered out
		"bulk_like":   true,
		"search_text": true,
	}
	for _, set := range modeToolSets {
		for name := range set {
			names[name] = true
		}
	}
	for name := range names {
		r.infos[name] = &schema.ToolInfo{Name: name}
	}
	return r
}

func toolNameSet(infos []*schema.ToolInfo) map[string]bool {
	out := make(map[string]bool, len(infos))
	for _, info := range infos {
		out[info.Name] = true
	}
	return out
}

func TestGetToolInfosByModeReturnsModeSubset(t *testing.T) {
	r := newTestRegistry()
	for mode, want := range modeToolSets {
		got := toolNameSet(r.GetToolInfosByMode(mode))
		if len(got) != len(want) {
			t.Errorf("mode %q: got %d tools, want %d", mode, len(got), len(want))
		}
		for name := range want {
			if !got[name] {
				t.Errorf("mode %q: missing tool %q", mode, name)
			}
		}
		// A free-mode-only tool must never leak into a constrained mode.
		if got["bulk_like"] {
			t.Errorf("mode %q: bulk_like leaked into constrained mode", mode)
		}
	}
}

func TestGetToolInfosByModeEmptyOrUnknownReturnsAll(t *testing.T) {
	r := newTestRegistry()
	all := len(r.infos)
	if n := len(r.GetToolInfosByMode("")); n != all {
		t.Errorf("empty mode: got %d tools, want all %d", n, all)
	}
	if n := len(r.GetToolInfosByMode("nonsense")); n != all {
		t.Errorf("unknown mode: got %d tools, want all %d", n, all)
	}
}

func TestModeHasTool(t *testing.T) {
	if !ModeHasTool("", "tag_assets") {
		t.Error("free mode should have every tool")
	}
	if !ModeHasTool("nonsense", "tag_assets") {
		t.Error("unknown mode should fall back to full toolset")
	}
	if ModeHasTool("curate", "tag_assets") {
		t.Error("curate must not expose tag_assets")
	}
	if !ModeHasTool("organize", "peek") {
		t.Error("organize should expose peek (place/person grouping verification)")
	}
	if !ModeHasTool("curate", "dedupe") {
		t.Error("curate should expose dedupe")
	}
}

func TestBuildInstructionGatesToolMentions(t *testing.T) {
	// curate excludes tag_assets → no ORGANIZING/tag_assets guidance.
	curate := buildInstruction("Mon, 2026-01-01", nil, "curate")
	if strings.Contains(curate, "tag_assets") {
		t.Error("curate instruction must not mention tag_assets")
	}
	// organize includes tag_assets → guidance present.
	organize := buildInstruction("Mon, 2026-01-01", nil, "organize")
	if !strings.Contains(organize, "tag_assets") {
		t.Error("organize instruction should mention tag_assets")
	}
	// `top` was removed from the producer/consumer example list everywhere.
	if strings.Contains(curate, " top,") || strings.Contains(organize, " top,") {
		t.Error("instruction example list should no longer name top")
	}
}

func TestModeInstruction(t *testing.T) {
	if got := ModeInstruction(""); got != "" {
		t.Errorf("free mode instruction = %q, want empty", got)
	}
	if got := ModeInstruction("nonsense"); got != "" {
		t.Errorf("unknown mode instruction = %q, want empty", got)
	}
	for mode := range modeToolSets {
		got := ModeInstruction(mode)
		if got == "" {
			t.Errorf("mode %q: instruction is empty", mode)
		}
		if !strings.HasPrefix(got, "\n") {
			t.Errorf("mode %q: instruction should be newline-prefixed for appending", mode)
		}
	}
}
