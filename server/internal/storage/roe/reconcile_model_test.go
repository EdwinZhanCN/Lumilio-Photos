package roe_test

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"server/internal/storage/roe"
	"server/internal/storage/roe/reference"
)

func TestReducerMatchesReferenceAcrossReorderingDuplicatesAndRestart(t *testing.T) {
	t.Parallel()

	base := modelCampaign()
	for seed := int64(1); seed <= 64; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			t.Parallel()
			rng := rand.New(rand.NewSource(seed))
			deliveries := append([]roe.Event(nil), base...)
			rng.Shuffle(len(deliveries), func(i, j int) {
				deliveries[i], deliveries[j] = deliveries[j], deliveries[i]
			})
			// A canonical replay follows the disorder so work that arrived before
			// its prerequisite is retried and the sequence must converge.
			deliveries = append(deliveries, base...)

			production := roe.NewState()
			model := reference.New()
			for index, event := range deliveries {
				productionResult := production.Apply(event)
				modelResult := model.Apply(event)
				if !reflect.DeepEqual(productionResult, modelResult) {
					t.Fatalf("event %d result mismatch\nproduction=%+v\nreference=%+v", index, productionResult, modelResult)
				}
				if !reflect.DeepEqual(production.Snapshot(), model.Snapshot()) {
					t.Fatalf("event %d state mismatch after %+v\nproduction=%+v\nreference=%+v", index, event, production.Snapshot(), model.Snapshot())
				}

				// Crash after every durable transition, restore, and replay the
				// delivered event. The replay must be an exact no-op.
				beforeCrash := production.Snapshot()
				production = roe.Restore(beforeCrash)
				production.Apply(event)
				if !reflect.DeepEqual(beforeCrash, production.Snapshot()) {
					t.Fatalf("event %d changed after crash replay: %+v", index, event)
				}
			}

			assertCampaignConverged(t, production.Snapshot())
		})
	}
}

func TestReducerRejectsActiveNameAndBindingInvariantViolations(t *testing.T) {
	t.Parallel()

	state := roe.NewState()
	if result := state.Apply(observeDirectory("root", 1, "root", "", "", "")); result.Code != roe.ResultApplied {
		t.Fatalf("root observation result = %+v", result)
	}
	first := observe("observe-a", 1, "a", "root", "Photo.JPG", "photo.jpg", "token-a")
	if result := state.Apply(first); result.Code != roe.ResultApplied {
		t.Fatalf("first observation result = %+v", result)
	}
	conflict := observe("observe-b", 2, "b", "root", "photo.jpg", "photo.jpg", "token-b")
	if result := state.Apply(conflict); result.Code != roe.ResultConflict {
		t.Fatalf("duplicate normalized child result = %+v, want conflict", result)
	}

	state.Apply(hash("hash-a", "a", 1, 7, "token-a", "same-bytes", 11))
	state.Apply(hash("hash-a-replay", "a", 1, 7, "token-a", "same-bytes", 11))
	snapshot := state.Snapshot()
	if len(snapshot.ActiveLocations) != 1 || len(snapshot.Assets) != 1 || len(snapshot.Outbox) != 1 {
		t.Fatalf("idempotent binding snapshot = %+v", snapshot)
	}
}

func modelCampaign() []roe.Event {
	events := []roe.Event{
		observeDirectory("root", 1, "root", "", "", ""),
		observeDirectory("observe-trips", 2, "trips", "root", "Trips", "trips"),
		observe("observe-a-v1", 3, "a", "trips", "a.jpg", "a.jpg", "a-v1"),
		hash("hash-a-v1", "a", 3, 7, "a-v1", "same-bytes", 11),
		observe("observe-copy", 4, "copy", "root", "copy.jpg", "copy.jpg", "copy-v1"),
		hash("hash-copy", "copy", 4, 7, "copy-v1", "same-bytes", 11),
		observe("observe-other-owner-copy", 5, "owner-copy", "root", "owner-copy.jpg", "owner-copy.jpg", "owner-copy-v1"),
		hash("hash-other-owner", "owner-copy", 5, 8, "owner-copy-v1", "same-bytes", 11),
		{
			Key:              "rename-trips",
			Kind:             roe.EventRename,
			Revision:         5,
			NodeID:           "trips",
			ParentID:         "root",
			Name:             "Archive",
			NameKey:          "archive",
			ExpectedRevision: 2,
		},
		observe("observe-a-v2", 6, "a", "trips", "a.jpg", "a.jpg", "a-v2"),
		hash("stale-hash-a-v1", "a", 3, 7, "a-v1", "same-bytes", 11),
		hash("hash-a-v2", "a", 6, 7, "a-v2", "new-bytes", 19),
		{Key: "delete-hint-copy", Kind: roe.EventDeleteHint, Revision: 7, NodeID: "copy", ParentID: "root"},
		{Key: "overflow", Kind: roe.EventCursorGap, Revision: 8},
		{
			Key:              "unsafe-finalize",
			Kind:             roe.EventFinalizeDirectory,
			Revision:         9,
			NodeID:           "root",
			Authoritative:    true,
			CursorHealthy:    false,
			ObservedChildren: []roe.NodeID{"trips"},
		},
		{Key: "cursor-recovered", Kind: roe.EventCursorHealthy, Revision: 10},
		{
			Key:              "safe-finalize",
			Kind:             roe.EventFinalizeDirectory,
			Revision:         11,
			NodeID:           "root",
			Authoritative:    true,
			CursorHealthy:    true,
			ObservedChildren: []roe.NodeID{"trips"},
		},
	}
	for index := range events {
		events[index].Sequence = uint64(index + 1)
	}
	return events
}

func observe(key string, revision uint64, nodeID, parentID, name, nameKey, token string) roe.Event {
	return roe.Event{
		Key:            key,
		Kind:           roe.EventObserve,
		Revision:       revision,
		NodeID:         roe.NodeID(nodeID),
		ParentID:       roe.NodeID(parentID),
		Name:           name,
		NameKey:        nameKey,
		NodeKind:       roe.NodeFile,
		StabilityToken: token,
	}
}

func observeDirectory(key string, revision uint64, nodeID, parentID, name, nameKey string) roe.Event {
	event := observe(key, revision, nodeID, parentID, name, nameKey, "directory")
	event.NodeKind = roe.NodeDirectory
	return event
}

func hash(key, nodeID string, revision uint64, ownerID int32, token, fullHash string, size int64) roe.Event {
	return roe.Event{
		Key:                  key,
		Kind:                 roe.EventHashStable,
		Revision:             revision,
		NodeID:               roe.NodeID(nodeID),
		ExpectedRevision:     revision,
		OwnerID:              ownerID,
		HashAlgorithm:        "blake3-v1",
		FullHash:             fullHash,
		FileSize:             size,
		StabilityTokenBefore: token,
		StabilityTokenAfter:  token,
	}
}

func assertCampaignConverged(t *testing.T, snapshot roe.Snapshot) {
	t.Helper()
	if snapshot.FullVerificationRequired {
		t.Fatal("healthy authoritative verification did not clear full-verification requirement")
	}
	if len(snapshot.Contents) != 2 {
		t.Fatalf("content count = %d, want 2: %+v", len(snapshot.Contents), snapshot)
	}
	if len(snapshot.Assets) != 3 {
		t.Fatalf("owner/content asset count = %d, want 3: %+v", len(snapshot.Assets), snapshot)
	}
	if len(snapshot.ActiveLocations) != 1 || snapshot.ActiveLocations[0].NodeID != "a" {
		t.Fatalf("active locations = %+v, want only replacement at node a", snapshot.ActiveLocations)
	}
	if len(snapshot.Outbox) != 3 {
		t.Fatalf("processing outbox effects = %d, want one per new Asset", len(snapshot.Outbox))
	}
	for _, node := range snapshot.Nodes {
		if node.NodeID == "trips" && (node.Name != "Archive" || node.Revision != 5) {
			t.Fatalf("directory rename was not one graph-edge update: %+v", node)
		}
	}
}
