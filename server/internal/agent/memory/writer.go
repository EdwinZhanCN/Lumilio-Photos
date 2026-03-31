package memory

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
)

type Store interface {
	EnsureCollection(ctx context.Context, dimensions int) error
	UpsertEpisodes(ctx context.Context, episodes []Episode, vectors [][]float32) error
	SearchEpisodes(ctx context.Context, request SearchRequest, queryVector []float32) ([]SearchHit, error)
}

type Embedder interface {
	Dimension() int
	EmbedText(ctx context.Context, text string) ([]float32, error)
}

type WritePolicy struct {
	WriteOnSuccess   bool
	WriteOnFailure   bool
	WriteOnRecovered bool
	MinToolSteps     int
	RequireSummary   bool
}

func DefaultWritePolicy() WritePolicy {
	return WritePolicy{
		WriteOnSuccess:   true,
		WriteOnFailure:   true,
		WriteOnRecovered: true,
		MinToolSteps:     1,
		RequireSummary:   true,
	}
}

func (p WritePolicy) ShouldWrite(episode Episode) bool {
	if p.RequireSummary && strings.TrimSpace(episode.Summary) == "" {
		return false
	}
	if len(episode.ToolTrace) < p.MinToolSteps {
		return false
	}

	switch episode.Status {
	case EpisodeStatusSucceeded:
		return p.WriteOnSuccess
	case EpisodeStatusFailed:
		return p.WriteOnFailure
	case EpisodeStatusRecovered:
		return p.WriteOnRecovered
	default:
		return false
	}
}

type Writer struct {
	store    Store
	embedder Embedder
	policy   WritePolicy
}

func NewWriter(store Store, embedder Embedder, policy WritePolicy) *Writer {
	return &Writer{
		store:    store,
		embedder: embedder,
		policy:   policy,
	}
}

func (w *Writer) Ensure(ctx context.Context) error {
	if w.store == nil {
		return errors.New("memory writer store is nil")
	}
	if w.embedder == nil {
		return errors.New("memory writer embedder is nil")
	}
	return w.store.EnsureCollection(ctx, w.embedder.Dimension())
}

func (w *Writer) WriteEpisode(ctx context.Context, episode Episode) error {
	if w.store == nil {
		return errors.New("memory writer store is nil")
	}
	if w.embedder == nil {
		return errors.New("memory writer embedder is nil")
	}
	if !w.policy.ShouldWrite(episode) {
		return nil
	}
	if episode.ID == "" {
		episode.ID = uuid.NewString()
	}
	if strings.TrimSpace(episode.RetrievalText) == "" {
		episode.RetrievalText = episode.BuildRetrievalText()
	}

	vector, err := w.embedder.EmbedText(ctx, episode.RetrievalText)
	if err != nil {
		return err
	}
	return w.store.UpsertEpisodes(ctx, []Episode{episode}, [][]float32{vector})
}

func (w *Writer) Search(ctx context.Context, request SearchRequest) ([]SearchHit, error) {
	if w.store == nil {
		return nil, errors.New("memory writer store is nil")
	}
	if w.embedder == nil {
		return nil, errors.New("memory writer embedder is nil")
	}
	if strings.TrimSpace(request.Query) == "" {
		return nil, errors.New("search query is empty")
	}

	vector, err := w.embedder.EmbedText(ctx, request.Query)
	if err != nil {
		return nil, err
	}
	return w.store.SearchEpisodes(ctx, request, vector)
}
