package main

import (
	"strings"
	"testing"
)

// movecheck pairs functions by receiver type and name with package
// qualifiers and export capitalization folded: a function that moved
// packages and was renamed only in case compares unchanged; a changed body,
// a rename with the same body, and real additions and removals are listed.
func TestMovecheckSeparatesMovesFromChanges(t *testing.T) {
	base := map[string][]byte{
		"server/internal/service/face.go": []byte(`package service

import "server/internal/helper"

// Apply is documented.
func (s *FaceService) Apply(x int) error { return helper.Do(x) }

func (s *FaceService) Count() int { return 1 }

func New() *FaceService { return &FaceService{} }

func oldName(a int) int { return a * 2 }

func gone() {}
`),
		"server/internal/music/music.go": []byte(`package music

func New() *Service { return &Service{} }
`),
	}
	head := map[string][]byte{
		"server/internal/people/service.go": []byte(`package people

import "server/internal/helper"

// apply lost its export and its package; the body is the same.
func (s *faceService) apply(x int) error {
	return helper.Do(x)
}

func (s *faceService) Count() int { return 2 }

func New() *faceService { return &faceService{} }

func newName(a int) int { return a * 2 }

func added() {}
`),
		"server/internal/music/music.go": []byte(`package music

func New() *Service { return &Service{} }
`),
	}
	baseFuncs, err := indexFuncs(base)
	if err != nil {
		t.Fatal(err)
	}
	headFuncs, err := indexFuncs(head)
	if err != nil {
		t.Fatal(err)
	}
	result := compareFuncs(baseFuncs, headFuncs)
	report := result.render("origin/dev")
	for _, want := range []string{
		"1 changed · 1 renamed · 2 moved unchanged · 1 added · 1 removed · 1 untouched",
		"- `FaceService.Count → faceService.Count` `server/internal/service/face.go` → `server/internal/people/service.go` — reason:",
		"- `oldName` → `newName`",
		"## Added\n\n- `added`",
		"## Removed\n\n- `gone`",
		"- `faceService.apply` `server/internal/service/face.go` → `server/internal/people/service.go`",
		"- `New` `server/internal/service/face.go` → `server/internal/people/service.go`",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}
