package state

import (
	"testing"

	"desktop/internal/control/dto"
)

func TestCommitOnlyAdvancesWhenSnapshotChanges(t *testing.T) {
	store := NewWithInstanceID("instance")
	ch, cancel := store.Subscribe(1)
	defer cancel()

	if _, changed := store.Commit(func(snapshot *dto.DesktopSnapshot) {}); changed {
		t.Fatal("empty reducer changed snapshot")
	}
	if _, changed := store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Host.BootPhase = "ready"
	}); !changed {
		t.Fatal("changed reducer did not commit")
	}

	snapshot := store.Get()
	if snapshot.Revision != 2 || snapshot.Host.BootPhase != "ready" {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	notice := <-ch
	if notice.InstanceID != "instance" || notice.Revision != 2 {
		t.Fatalf("unexpected notice: %+v", notice)
	}
}

func TestSubscriptionIsLatestOnly(t *testing.T) {
	store := NewWithInstanceID("instance")
	ch, cancel := store.Subscribe(1)
	defer cancel()
	for i := 0; i < 20; i++ {
		store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Host.BootPhase = string(rune('a' + i))
		})
	}
	notice := <-ch
	if notice.Revision != 21 {
		t.Fatalf("latest-only subscription retained revision %d, want 21", notice.Revision)
	}
}

func TestSnapshotIsCopiedAtCommitAndRead(t *testing.T) {
	store := NewWithInstanceID("instance")
	input := []dto.OperationSnapshot{{OperationID: "op"}}
	store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = input
	})
	input[0].OperationID = "mutated"
	read := store.Get()
	read.Operations[0].OperationID = "read-mutated"
	if got := store.Get().Operations[0].OperationID; got != "op" {
		t.Fatalf("snapshot was mutable through caller: %q", got)
	}
}
