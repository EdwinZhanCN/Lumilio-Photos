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
