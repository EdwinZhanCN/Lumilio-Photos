// Package event owns deterministic media Event semantics.
package event

import "time"

const AlgorithmVersion = "events-v1"

type Coordinate struct {
	Latitude  float64
	Longitude float64
}

type Candidate struct {
	MediaItemID string
	CapturedAt  time.Time
	TimeSource  string
	Timezone    string
	Coordinate  *Coordinate
	StackID     string
}

type ConstraintKind string

const (
	ConstraintInclude    ConstraintKind = "include"
	ConstraintExclude    ConstraintKind = "exclude"
	ConstraintMustLink   ConstraintKind = "must_link"
	ConstraintCannotLink ConstraintKind = "cannot_link"
)

type Constraint struct {
	Kind             ConstraintKind
	EventID          string
	LeftMediaItemID  string
	RightMediaItemID string
}

type Segment struct {
	MediaItemIDs []string
	StartAt      time.Time
	EndAt        time.Time
	Timezone     string
	Coordinate   *Coordinate
	HardEventID  string
}

type PublishedEvent struct {
	EventID         string
	MediaItemIDs    []string
	StartAt         time.Time
	EndAt           time.Time
	Coordinate      *Coordinate
	CoverOverrideID string
	TitleOverride   *string
	Hidden          bool
	HasUserState    bool
	CreatedAt       time.Time
}

type Assignment struct {
	EventID      string
	SegmentIndex int
	Reused       bool
}

type Redirect struct {
	OldEventID string
	NewEventID string
}

type ReconcileResult struct {
	Assignments []Assignment
	Redirects   []Redirect
}
