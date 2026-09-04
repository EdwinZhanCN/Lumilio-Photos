package search

import (
	"context"
	"errors"
	"testing"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

func TestCosineFromL2(t *testing.T) {
	t.Parallel()

	if got := CosineFromL2(0); got != 1 {
		t.Fatalf("identical vectors: cosine = %v, want 1", got)
	}
	if got := CosineFromL2(1.224744871); got < 0.249 || got > 0.251 {
		t.Fatalf("cosine at floor distance = %v, want ~0.25", got)
	}
}

func TestFilterByCosineFloorDropsUnrelatedTail(t *testing.T) {
	t.Parallel()

	near := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	far := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	kept := FilterByCosineFloor([]Candidate{
		{AssetID: near, RawScore: 0.1},
		{AssetID: far, RawScore: 1.4},
	}, ImageQueryCosineFloor)
	if len(kept) != 1 || kept[0].AssetID != near {
		t.Fatalf("filtered = %+v, want near asset only", kept)
	}
}

func TestResolveQuerySpaceUsesPrecomputedEmbedding(t *testing.T) {
	t.Parallel()

	embedCalled := false
	retriever := &EmbeddingRetriever{
		embed: func(context.Context, string, bool) (QueryEmbedding, error) {
			embedCalled = true
			return QueryEmbedding{}, errors.New("semantic_text_embed must not run")
		},
		resolveSpace: func(context.Context, string, int) (repo.EmbeddingSpace, error) {
			return repo.EmbeddingSpace{ID: 1, ModelID: "fixture", Dimensions: 768}, nil
		},
	}

	vector := make([]float32, 768)
	vector[0] = 1
	embedding, space, err := retriever.resolveQuerySpace(context.Background(), Request{
		Query: "must-not-embed",
		QueryEmbedding: &QueryEmbedding{
			Model:  "fixture",
			Vector: vector,
		},
	})
	if err != nil {
		t.Fatalf("resolveQuerySpace: %v", err)
	}
	if embedCalled {
		t.Fatal("expected precomputed QueryEmbedding to skip text embed")
	}
	if embedding.Model != "fixture" || len(embedding.Vector) != 768 {
		t.Fatalf("embedding = %+v", embedding)
	}
	if space.ID != 1 {
		t.Fatalf("space ID = %d, want 1", space.ID)
	}
}
