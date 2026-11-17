package trash

import (
	"reflect"
	"testing"
)

// The follow-up actions of a rejection live with the reasons, so a handler
// never invents them: only an offline Repository offers reconnect and only a
// changed file offers rescan.
func TestRejectionActionsAreOwnedByTheReason(t *testing.T) {
	cases := map[string][]string{
		ReasonRepositoryOffline: {"reconnect"},
		ReasonFileChanged:       {"rescan"},
		ReasonAssetMissing:      {},
		ReasonNotTrashed:        {},
		ReasonTrashFileMissing:  {},
		ReasonMoveFailed:        {},
	}
	for reason, want := range cases {
		if got := (&Rejection{Reason: reason}).Actions(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Rejection{%s}.Actions() = %v, want %v", reason, got, want)
		}
	}
}
