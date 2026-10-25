package storage

import (
	"reflect"
	"testing"
)

// The resolution codes the contract exposes are projected from the storage
// conflict actions here and nowhere else; an action the commands do not
// accept projects to nothing.
func TestHostActionResolutionsProjectTheClosedVocabulary(t *testing.T) {
	cases := []struct {
		actions []string
		want    []string
	}{
		{actions: []string{"relocate", "copy"}, want: []string{"update_location", "add_separate"}},
		{actions: []string{"copy"}, want: []string{"add_separate"}},
		{actions: []string{"confirm_risk"}, want: []string{"confirm_risk"}},
		{actions: []string{"open", "diagnose", "rename"}, want: []string{}},
		{actions: nil, want: []string{}},
	}
	for _, tc := range cases {
		if got := HostActionResolutions(tc.actions); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("HostActionResolutions(%v) = %v, want %v", tc.actions, got, tc.want)
		}
	}
	if got := (&ExistingRepositoryFoundError{}).Actions(); !reflect.DeepEqual(got, []string{"open"}) {
		t.Fatalf("ExistingRepositoryFoundError.Actions() = %v", got)
	}
	if got := (&RepositoryMarkerInvalidError{}).Actions(); !reflect.DeepEqual(got, []string{"diagnose"}) {
		t.Fatalf("RepositoryMarkerInvalidError.Actions() = %v", got)
	}
}
