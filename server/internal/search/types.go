package search

import (
	"context"
	"errors"
	"time"

	"server/internal/db/repo"

	"github.com/google/uuid"
)

const (
	SourceEmbedding = "embedding"
	SourceOCR       = "ocr"
	SourcePlace     = "place"
)

var ErrEmptyQuery = errors.New("aggregate search query is empty")

type Filter struct {
	AssetIDs         []uuid.UUID
	RepositoryID     *uuid.UUID
	PersonID         *int32
	AssetType        *string
	AssetTypes       []string
	OwnerID          *int32
	AlbumID          *int32
	FilenameValue    *string
	FilenameOperator *string
	DateFrom         *time.Time
	DateTo           *time.Time
	IsRaw            *bool
	IsDeleted        *bool
	Rating           *int
	Liked            *bool
	CameraModel      *string
	LensModel        *string
	TagName          *string
	TagSource        *string
	TagNames         []string
	LocationNorth    *float64
	LocationSouth    *float64
	LocationEast     *float64
	LocationWest     *float64
}

type Request struct {
	Query string
	Filter
	Limit      int
	Offset     int
	TopK       int
	CountTotal bool
	Debug      bool
}

type Response struct {
	Assets            []repo.Asset
	TotalCandidates   int
	CandidatePoolSize int
	Sources           []SourceMeta
	Debug             []AssetDebug
}

type SourceMeta struct {
	Type           string        `json:"type"`
	Weight         float64       `json:"weight"`
	CandidateCount int           `json:"candidate_count"`
	Duration       time.Duration `json:"-"`
	DurationMs     int64         `json:"duration_ms"`
	Error          string        `json:"error,omitempty"`
}

type Contribution struct {
	Rank     int     `json:"rank"`
	Weight   float64 `json:"weight"`
	RRFScore float64 `json:"rrf_score"`
	RawScore float64 `json:"raw_score"`
}

type AssetDebug struct {
	AssetID       string                  `json:"asset_id"`
	Score         float64                 `json:"score"`
	Contributions map[string]Contribution `json:"contributions"`
}

type Candidate struct {
	AssetID  uuid.UUID
	Source   string
	Rank     int
	RawScore float64
}

type Retriever interface {
	Source() string
	Weight() float64
	Retrieve(ctx context.Context, req Request) ([]Candidate, error)
}

type QueryEmbedding struct {
	Model  string
	Vector []float32
}

type EmbedQueryFunc func(ctx context.Context, query string, fast bool) (QueryEmbedding, error)
type ResolveEmbeddingSpaceFunc func(ctx context.Context, model string, dimensions int) (repo.EmbeddingSpace, error)
