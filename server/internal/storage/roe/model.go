// Package roe contains the deterministic state machine for repository
// observations. It deliberately has no filesystem, SQL, or River dependency;
// adapters persist events and project these decisions in bounded transactions.
package roe

import (
	"fmt"
	"sort"
	"strings"
)

type NodeID string

type NodeKind string

const (
	NodeDirectory NodeKind = "directory"
	NodeFile      NodeKind = "file"
	NodeSymlink   NodeKind = "symlink"
)

type EventKind string

const (
	EventObserve           EventKind = "observe"
	EventRename            EventKind = "rename"
	EventHashStable        EventKind = "hash_stable"
	EventDeleteHint        EventKind = "delete_hint"
	EventFinalizeDirectory EventKind = "finalize_directory"
	EventCursorGap         EventKind = "cursor_gap"
	EventCursorHealthy     EventKind = "cursor_healthy"
	EventRepositoryOffline EventKind = "repository_offline"
	EventCancel            EventKind = "cancel"
)

type Event struct {
	Key                  string
	Kind                 EventKind
	Sequence             uint64
	Revision             uint64
	NodeID               NodeID
	ParentID             NodeID
	Name                 string
	NameKey              string
	NodeKind             NodeKind
	StabilityToken       string
	ExpectedRevision     uint64
	OwnerID              int32
	HashAlgorithm        string
	FullHash             string
	FileSize             int64
	StabilityTokenBefore string
	StabilityTokenAfter  string
	Authoritative        bool
	CoverageError        bool
	CursorHealthy        bool
	ObservedChildren     []NodeID
}

type ResultCode string

const (
	ResultApplied  ResultCode = "applied"
	ResultNoop     ResultCode = "noop"
	ResultStale    ResultCode = "stale"
	ResultDeferred ResultCode = "deferred"
	ResultConflict ResultCode = "conflict"
)

type Result struct {
	Code   ResultCode
	Reason string
}

type Node struct {
	NodeID         NodeID
	ParentID       NodeID
	Name           string
	NameKey        string
	Kind           NodeKind
	Revision       uint64
	StabilityToken string
	Active         bool
}

type Content struct {
	ContentID     string
	HashAlgorithm string
	FullHash      string
	FileSize      int64
}

type Asset struct {
	AssetID   string
	OwnerID   int32
	ContentID string
}

type Location struct {
	LocationID      string
	NodeID          NodeID
	AssetID         string
	BoundRevision   uint64
	UnboundRevision uint64
}

type Effect struct {
	Key              string
	AssetID          string
	ContentID        string
	ExpectedRevision uint64
}

type Snapshot struct {
	NextSequence             uint64
	CursorRevision           uint64
	FullVerificationRequired bool
	Nodes                    []Node
	Contents                 []Content
	Assets                   []Asset
	Locations                []Location
	ActiveLocations          []Location
	Outbox                   []Effect
	AppliedKeys              []string
	Pending                  []Event
	Coverage                 map[NodeID]uint64
}

type State struct {
	nextSequence             uint64
	cursorRevision           uint64
	fullVerificationRequired bool
	nodes                    map[NodeID]Node
	activeNames              map[string]NodeID
	contents                 map[string]Content
	assets                   map[string]Asset
	locations                []Location
	activeLocations          map[NodeID]int
	outbox                   map[string]Effect
	appliedKeys              map[string]struct{}
	pendingKeys              map[string]struct{}
	pending                  map[uint64]Event
	coverage                 map[NodeID]uint64
}

func NewState() *State {
	return &State{
		nextSequence:    1,
		nodes:           make(map[NodeID]Node),
		activeNames:     make(map[string]NodeID),
		contents:        make(map[string]Content),
		assets:          make(map[string]Asset),
		activeLocations: make(map[NodeID]int),
		outbox:          make(map[string]Effect),
		appliedKeys:     make(map[string]struct{}),
		pendingKeys:     make(map[string]struct{}),
		pending:         make(map[uint64]Event),
		coverage:        make(map[NodeID]uint64),
	}
}

func Restore(snapshot Snapshot) *State {
	state := NewState()
	state.nextSequence = snapshot.NextSequence
	if state.nextSequence == 0 {
		state.nextSequence = 1
	}
	state.cursorRevision = snapshot.CursorRevision
	state.fullVerificationRequired = snapshot.FullVerificationRequired
	for _, node := range snapshot.Nodes {
		state.nodes[node.NodeID] = node
		if node.Active && node.ParentID != "" {
			state.activeNames[childKey(node.ParentID, node.NameKey)] = node.NodeID
		}
	}
	for _, content := range snapshot.Contents {
		state.contents[contentKey(content.HashAlgorithm, content.FullHash, content.FileSize)] = content
	}
	for _, asset := range snapshot.Assets {
		state.assets[assetKey(asset.OwnerID, asset.ContentID)] = asset
	}
	state.locations = append(state.locations, snapshot.Locations...)
	for index := range state.locations {
		if state.locations[index].UnboundRevision == 0 {
			state.activeLocations[state.locations[index].NodeID] = index
		}
	}
	for _, effect := range snapshot.Outbox {
		state.outbox[effect.Key] = effect
	}
	for _, key := range snapshot.AppliedKeys {
		state.appliedKeys[key] = struct{}{}
	}
	for _, event := range snapshot.Pending {
		state.pending[event.Sequence] = event
		state.pendingKeys[event.Key] = struct{}{}
	}
	for nodeID, revision := range snapshot.Coverage {
		state.coverage[nodeID] = revision
	}
	return state
}

func (s *State) Apply(event Event) Result {
	if s == nil {
		return Result{Code: ResultConflict, Reason: "state is nil"}
	}
	if strings.TrimSpace(event.Key) == "" {
		return Result{Code: ResultConflict, Reason: "event key is required"}
	}
	if _, ok := s.appliedKeys[event.Key]; ok {
		return Result{Code: ResultNoop, Reason: "event already applied"}
	}
	if _, ok := s.pendingKeys[event.Key]; ok {
		return Result{Code: ResultNoop, Reason: "event already buffered"}
	}
	if event.Sequence == 0 {
		result := s.applyOne(event)
		if terminalResult(result.Code) {
			s.appliedKeys[event.Key] = struct{}{}
		}
		return result
	}
	if event.Sequence < s.nextSequence {
		return Result{Code: ResultStale, Reason: "sequence already passed"}
	}
	if event.Sequence > s.nextSequence {
		s.pending[event.Sequence] = cloneEvent(event)
		s.pendingKeys[event.Key] = struct{}{}
		return Result{Code: ResultDeferred, Reason: "waiting for earlier revision"}
	}

	result := s.applyOne(event)
	if !terminalResult(result.Code) {
		return result
	}
	s.appliedKeys[event.Key] = struct{}{}
	s.nextSequence++
	for {
		pending, ok := s.pending[s.nextSequence]
		if !ok {
			break
		}
		delete(s.pending, s.nextSequence)
		delete(s.pendingKeys, pending.Key)
		pendingResult := s.applyOne(pending)
		if !terminalResult(pendingResult.Code) {
			s.pending[s.nextSequence] = pending
			s.pendingKeys[pending.Key] = struct{}{}
			break
		}
		s.appliedKeys[pending.Key] = struct{}{}
		s.nextSequence++
	}
	return result
}

func (s *State) applyOne(event Event) Result {
	switch event.Kind {
	case EventObserve:
		return s.applyObserve(event)
	case EventRename:
		return s.applyRename(event)
	case EventHashStable:
		return s.applyHash(event)
	case EventFinalizeDirectory:
		return s.applyFinalize(event)
	case EventCursorGap:
		if event.Revision >= s.cursorRevision {
			s.cursorRevision = event.Revision
			s.fullVerificationRequired = true
			return Result{Code: ResultApplied}
		}
		return Result{Code: ResultStale, Reason: "older cursor state"}
	case EventCursorHealthy:
		if event.Revision >= s.cursorRevision {
			s.cursorRevision = event.Revision
			return Result{Code: ResultApplied}
		}
		return Result{Code: ResultStale, Reason: "older cursor state"}
	case EventDeleteHint, EventRepositoryOffline, EventCancel:
		return Result{Code: ResultNoop, Reason: "hint cannot finalize catalog state"}
	default:
		return Result{Code: ResultConflict, Reason: fmt.Sprintf("unsupported event kind %q", event.Kind)}
	}
}

func (s *State) applyObserve(event Event) Result {
	if event.NodeID == "" || event.NodeKind == "" {
		return Result{Code: ResultConflict, Reason: "node identity and kind are required"}
	}
	if event.ParentID != "" {
		parent, ok := s.nodes[event.ParentID]
		if !ok || !parent.Active || parent.Kind != NodeDirectory {
			return Result{Code: ResultDeferred, Reason: "parent directory is not active"}
		}
	}
	current, exists := s.nodes[event.NodeID]
	if exists && event.Revision <= current.Revision {
		return Result{Code: ResultStale, Reason: "node has a newer observation"}
	}
	key := childKey(event.ParentID, event.NameKey)
	if event.ParentID != "" {
		if occupied, ok := s.activeNames[key]; ok && occupied != event.NodeID {
			return Result{Code: ResultConflict, Reason: "active normalized child already exists"}
		}
	}
	if exists && current.Active && current.ParentID != "" {
		delete(s.activeNames, childKey(current.ParentID, current.NameKey))
	}
	if exists && current.StabilityToken != event.StabilityToken {
		s.closeLocation(event.NodeID, event.Revision)
	}
	s.nodes[event.NodeID] = Node{
		NodeID:         event.NodeID,
		ParentID:       event.ParentID,
		Name:           event.Name,
		NameKey:        event.NameKey,
		Kind:           event.NodeKind,
		Revision:       event.Revision,
		StabilityToken: event.StabilityToken,
		Active:         true,
	}
	if event.ParentID != "" {
		s.activeNames[key] = event.NodeID
	}
	return Result{Code: ResultApplied}
}

func (s *State) applyRename(event Event) Result {
	node, ok := s.nodes[event.NodeID]
	if !ok || !node.Active {
		return Result{Code: ResultDeferred, Reason: "renamed node is not active"}
	}
	if node.Revision != event.ExpectedRevision || event.Revision <= node.Revision {
		return Result{Code: ResultStale, Reason: "rename expected revision does not match"}
	}
	parent, ok := s.nodes[event.ParentID]
	if !ok || !parent.Active || parent.Kind != NodeDirectory {
		return Result{Code: ResultDeferred, Reason: "rename parent is not active"}
	}
	newKey := childKey(event.ParentID, event.NameKey)
	if occupied, exists := s.activeNames[newKey]; exists && occupied != event.NodeID {
		return Result{Code: ResultConflict, Reason: "rename target normalized child already exists"}
	}
	if node.ParentID != "" {
		delete(s.activeNames, childKey(node.ParentID, node.NameKey))
	}
	node.ParentID = event.ParentID
	node.Name = event.Name
	node.NameKey = event.NameKey
	node.Revision = event.Revision
	s.nodes[event.NodeID] = node
	s.activeNames[newKey] = event.NodeID
	return Result{Code: ResultApplied}
}

func (s *State) applyHash(event Event) Result {
	node, ok := s.nodes[event.NodeID]
	if !ok || !node.Active || node.Kind == NodeDirectory {
		return Result{Code: ResultDeferred, Reason: "hashed node is not an active file"}
	}
	if node.Revision != event.ExpectedRevision || event.Revision != event.ExpectedRevision {
		return Result{Code: ResultStale, Reason: "hash expected revision does not match"}
	}
	if event.StabilityTokenBefore == "" || event.StabilityTokenBefore != event.StabilityTokenAfter || event.StabilityTokenAfter != node.StabilityToken {
		return Result{Code: ResultStale, Reason: "file mutated while hashing"}
	}
	algorithm := strings.ToLower(strings.TrimSpace(event.HashAlgorithm))
	fullHash := strings.ToLower(strings.TrimSpace(event.FullHash))
	if event.OwnerID <= 0 || algorithm == "" || fullHash == "" || event.FileSize < 0 {
		return Result{Code: ResultConflict, Reason: "complete content identity and owner are required"}
	}
	contentLookup := contentKey(algorithm, fullHash, event.FileSize)
	content, contentExists := s.contents[contentLookup]
	if !contentExists {
		content = Content{
			ContentID:     "content:" + contentLookup,
			HashAlgorithm: algorithm,
			FullHash:      fullHash,
			FileSize:      event.FileSize,
		}
		s.contents[contentLookup] = content
	}
	assetLookup := assetKey(event.OwnerID, content.ContentID)
	asset, assetExists := s.assets[assetLookup]
	if !assetExists {
		asset = Asset{
			AssetID:   fmt.Sprintf("asset:%d:%s", event.OwnerID, content.ContentID),
			OwnerID:   event.OwnerID,
			ContentID: content.ContentID,
		}
		s.assets[assetLookup] = asset
		effect := Effect{
			Key:              "process:" + asset.AssetID,
			AssetID:          asset.AssetID,
			ContentID:        content.ContentID,
			ExpectedRevision: event.ExpectedRevision,
		}
		s.outbox[effect.Key] = effect
	}
	if index, exists := s.activeLocations[event.NodeID]; exists {
		current := s.locations[index]
		if current.AssetID == asset.AssetID && current.BoundRevision == event.ExpectedRevision {
			return Result{Code: ResultNoop, Reason: "binding already active"}
		}
		s.closeLocation(event.NodeID, event.ExpectedRevision)
	}
	location := Location{
		LocationID:    fmt.Sprintf("location:%s:%d", event.NodeID, event.ExpectedRevision),
		NodeID:        event.NodeID,
		AssetID:       asset.AssetID,
		BoundRevision: event.ExpectedRevision,
	}
	s.locations = append(s.locations, location)
	s.activeLocations[event.NodeID] = len(s.locations) - 1
	return Result{Code: ResultApplied}
}

func (s *State) applyFinalize(event Event) Result {
	directory, ok := s.nodes[event.NodeID]
	if !ok || !directory.Active || directory.Kind != NodeDirectory {
		return Result{Code: ResultDeferred, Reason: "covered directory is not active"}
	}
	if !event.Authoritative || event.CoverageError || !event.CursorHealthy {
		if event.CoverageError || !event.CursorHealthy {
			s.fullVerificationRequired = true
		}
		return Result{Code: ResultNoop, Reason: "coverage cannot finalize absence"}
	}
	observed := make(map[NodeID]struct{}, len(event.ObservedChildren))
	for _, child := range event.ObservedChildren {
		observed[child] = struct{}{}
	}
	for nodeID, node := range s.nodes {
		if !node.Active || node.ParentID != event.NodeID {
			continue
		}
		if _, present := observed[nodeID]; present {
			continue
		}
		delete(s.activeNames, childKey(node.ParentID, node.NameKey))
		node.Active = false
		if event.Revision > node.Revision {
			node.Revision = event.Revision
		}
		s.nodes[nodeID] = node
		s.closeLocation(nodeID, event.Revision)
	}
	s.coverage[event.NodeID] = event.Revision
	if event.Revision >= s.cursorRevision {
		s.cursorRevision = event.Revision
	}
	s.fullVerificationRequired = false
	return Result{Code: ResultApplied}
}

func (s *State) closeLocation(nodeID NodeID, revision uint64) {
	index, ok := s.activeLocations[nodeID]
	if !ok {
		return
	}
	location := s.locations[index]
	location.UnboundRevision = revision
	s.locations[index] = location
	delete(s.activeLocations, nodeID)
}

func (s *State) Snapshot() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	snapshot := Snapshot{
		NextSequence:             s.nextSequence,
		CursorRevision:           s.cursorRevision,
		FullVerificationRequired: s.fullVerificationRequired,
		Locations:                append([]Location(nil), s.locations...),
		Coverage:                 make(map[NodeID]uint64, len(s.coverage)),
	}
	for _, node := range s.nodes {
		snapshot.Nodes = append(snapshot.Nodes, node)
	}
	for _, content := range s.contents {
		snapshot.Contents = append(snapshot.Contents, content)
	}
	for _, asset := range s.assets {
		snapshot.Assets = append(snapshot.Assets, asset)
	}
	for _, index := range s.activeLocations {
		snapshot.ActiveLocations = append(snapshot.ActiveLocations, s.locations[index])
	}
	for _, effect := range s.outbox {
		snapshot.Outbox = append(snapshot.Outbox, effect)
	}
	for key := range s.appliedKeys {
		snapshot.AppliedKeys = append(snapshot.AppliedKeys, key)
	}
	for _, event := range s.pending {
		snapshot.Pending = append(snapshot.Pending, cloneEvent(event))
	}
	for nodeID, revision := range s.coverage {
		snapshot.Coverage[nodeID] = revision
	}
	sort.Slice(snapshot.Nodes, func(i, j int) bool { return snapshot.Nodes[i].NodeID < snapshot.Nodes[j].NodeID })
	sort.Slice(snapshot.Contents, func(i, j int) bool { return snapshot.Contents[i].ContentID < snapshot.Contents[j].ContentID })
	sort.Slice(snapshot.Assets, func(i, j int) bool { return snapshot.Assets[i].AssetID < snapshot.Assets[j].AssetID })
	sort.Slice(snapshot.Locations, func(i, j int) bool { return snapshot.Locations[i].LocationID < snapshot.Locations[j].LocationID })
	sort.Slice(snapshot.ActiveLocations, func(i, j int) bool {
		return snapshot.ActiveLocations[i].LocationID < snapshot.ActiveLocations[j].LocationID
	})
	sort.Slice(snapshot.Outbox, func(i, j int) bool { return snapshot.Outbox[i].Key < snapshot.Outbox[j].Key })
	sort.Strings(snapshot.AppliedKeys)
	sort.Slice(snapshot.Pending, func(i, j int) bool { return snapshot.Pending[i].Sequence < snapshot.Pending[j].Sequence })
	return snapshot
}

func terminalResult(code ResultCode) bool {
	return code != ResultDeferred
}

func childKey(parentID NodeID, nameKey string) string {
	return string(parentID) + "\x00" + strings.TrimSpace(nameKey)
}

func contentKey(algorithm, fullHash string, size int64) string {
	return fmt.Sprintf("%s:%s:%d", algorithm, fullHash, size)
}

func assetKey(ownerID int32, contentID string) string {
	return fmt.Sprintf("%d:%s", ownerID, contentID)
}

func cloneEvent(event Event) Event {
	event.ObservedChildren = append([]NodeID(nil), event.ObservedChildren...)
	return event
}
