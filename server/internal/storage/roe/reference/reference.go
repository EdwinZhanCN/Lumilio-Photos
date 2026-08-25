// Package reference is a deliberately simple, linear-search oracle for the
// ROE reducer. It favors obvious state transitions over production lookup
// structures so seeded campaigns can detect implementation drift.
package reference

import (
	"fmt"
	"sort"
	"strings"

	"server/internal/storage/roe"
)

type Model struct {
	nextSequence             uint64
	cursorRevision           uint64
	fullVerificationRequired bool
	nodes                    []roe.Node
	contents                 []roe.Content
	assets                   []roe.Asset
	locations                []roe.Location
	outbox                   []roe.Effect
	appliedKeys              []string
	pending                  []roe.Event
	coverage                 map[roe.NodeID]uint64
}

func New() *Model {
	return &Model{nextSequence: 1, coverage: make(map[roe.NodeID]uint64)}
}

func (m *Model) Apply(event roe.Event) roe.Result {
	if strings.TrimSpace(event.Key) == "" {
		return roe.Result{Code: roe.ResultConflict, Reason: "event key is required"}
	}
	if contains(m.appliedKeys, event.Key) {
		return roe.Result{Code: roe.ResultNoop, Reason: "event already applied"}
	}
	for _, item := range m.pending {
		if item.Key == event.Key {
			return roe.Result{Code: roe.ResultNoop, Reason: "event already buffered"}
		}
	}
	if event.Sequence == 0 {
		result := m.applyOne(event)
		if result.Code != roe.ResultDeferred {
			m.appliedKeys = append(m.appliedKeys, event.Key)
		}
		return result
	}
	if event.Sequence < m.nextSequence {
		return roe.Result{Code: roe.ResultStale, Reason: "sequence already passed"}
	}
	if event.Sequence > m.nextSequence {
		m.pending = append(m.pending, copyEvent(event))
		return roe.Result{Code: roe.ResultDeferred, Reason: "waiting for earlier revision"}
	}

	result := m.applyOne(event)
	if result.Code == roe.ResultDeferred {
		return result
	}
	m.appliedKeys = append(m.appliedKeys, event.Key)
	m.nextSequence++
	for {
		index := -1
		for i := range m.pending {
			if m.pending[i].Sequence == m.nextSequence {
				index = i
				break
			}
		}
		if index < 0 {
			break
		}
		pending := m.pending[index]
		m.pending = append(m.pending[:index], m.pending[index+1:]...)
		pendingResult := m.applyOne(pending)
		if pendingResult.Code == roe.ResultDeferred {
			m.pending = append(m.pending, pending)
			break
		}
		m.appliedKeys = append(m.appliedKeys, pending.Key)
		m.nextSequence++
	}
	return result
}

func (m *Model) applyOne(event roe.Event) roe.Result {
	switch event.Kind {
	case roe.EventObserve:
		return m.observe(event)
	case roe.EventRename:
		return m.rename(event)
	case roe.EventHashStable:
		return m.hash(event)
	case roe.EventFinalizeDirectory:
		return m.finalize(event)
	case roe.EventCursorGap:
		if event.Revision >= m.cursorRevision {
			m.cursorRevision = event.Revision
			m.fullVerificationRequired = true
			return roe.Result{Code: roe.ResultApplied}
		}
		return roe.Result{Code: roe.ResultStale, Reason: "older cursor state"}
	case roe.EventCursorHealthy:
		if event.Revision >= m.cursorRevision {
			m.cursorRevision = event.Revision
			return roe.Result{Code: roe.ResultApplied}
		}
		return roe.Result{Code: roe.ResultStale, Reason: "older cursor state"}
	case roe.EventDeleteHint, roe.EventRepositoryOffline, roe.EventCancel:
		return roe.Result{Code: roe.ResultNoop, Reason: "hint cannot finalize catalog state"}
	default:
		return roe.Result{Code: roe.ResultConflict, Reason: fmt.Sprintf("unsupported event kind %q", event.Kind)}
	}
}

func (m *Model) observe(event roe.Event) roe.Result {
	if event.NodeID == "" || event.NodeKind == "" {
		return roe.Result{Code: roe.ResultConflict, Reason: "node identity and kind are required"}
	}
	if event.ParentID != "" {
		parent, ok := m.node(event.ParentID)
		if !ok || !parent.Active || parent.Kind != roe.NodeDirectory {
			return roe.Result{Code: roe.ResultDeferred, Reason: "parent directory is not active"}
		}
	}
	current, exists := m.node(event.NodeID)
	if exists && event.Revision <= current.Revision {
		return roe.Result{Code: roe.ResultStale, Reason: "node has a newer observation"}
	}
	if event.ParentID != "" {
		for _, candidate := range m.nodes {
			if candidate.Active && candidate.ParentID == event.ParentID && candidate.NameKey == event.NameKey && candidate.NodeID != event.NodeID {
				return roe.Result{Code: roe.ResultConflict, Reason: "active normalized child already exists"}
			}
		}
	}
	if exists && current.StabilityToken != event.StabilityToken {
		m.closeLocation(event.NodeID, event.Revision)
	}
	next := roe.Node{
		NodeID:         event.NodeID,
		ParentID:       event.ParentID,
		Name:           event.Name,
		NameKey:        event.NameKey,
		Kind:           event.NodeKind,
		Revision:       event.Revision,
		StabilityToken: event.StabilityToken,
		Active:         true,
	}
	m.putNode(next)
	return roe.Result{Code: roe.ResultApplied}
}

func (m *Model) rename(event roe.Event) roe.Result {
	node, ok := m.node(event.NodeID)
	if !ok || !node.Active {
		return roe.Result{Code: roe.ResultDeferred, Reason: "renamed node is not active"}
	}
	if node.Revision != event.ExpectedRevision || event.Revision <= node.Revision {
		return roe.Result{Code: roe.ResultStale, Reason: "rename expected revision does not match"}
	}
	parent, ok := m.node(event.ParentID)
	if !ok || !parent.Active || parent.Kind != roe.NodeDirectory {
		return roe.Result{Code: roe.ResultDeferred, Reason: "rename parent is not active"}
	}
	for _, candidate := range m.nodes {
		if candidate.Active && candidate.ParentID == event.ParentID && candidate.NameKey == event.NameKey && candidate.NodeID != event.NodeID {
			return roe.Result{Code: roe.ResultConflict, Reason: "rename target normalized child already exists"}
		}
	}
	node.ParentID = event.ParentID
	node.Name = event.Name
	node.NameKey = event.NameKey
	node.Revision = event.Revision
	m.putNode(node)
	return roe.Result{Code: roe.ResultApplied}
}

func (m *Model) hash(event roe.Event) roe.Result {
	node, ok := m.node(event.NodeID)
	if !ok || !node.Active || node.Kind == roe.NodeDirectory {
		return roe.Result{Code: roe.ResultDeferred, Reason: "hashed node is not an active file"}
	}
	if node.Revision != event.ExpectedRevision || event.Revision != event.ExpectedRevision {
		return roe.Result{Code: roe.ResultStale, Reason: "hash expected revision does not match"}
	}
	if event.StabilityTokenBefore == "" || event.StabilityTokenBefore != event.StabilityTokenAfter || event.StabilityTokenAfter != node.StabilityToken {
		return roe.Result{Code: roe.ResultStale, Reason: "file mutated while hashing"}
	}
	algorithm := strings.ToLower(strings.TrimSpace(event.HashAlgorithm))
	fullHash := strings.ToLower(strings.TrimSpace(event.FullHash))
	if event.OwnerID <= 0 || algorithm == "" || fullHash == "" || event.FileSize < 0 {
		return roe.Result{Code: roe.ResultConflict, Reason: "complete content identity and owner are required"}
	}
	contentID := fmt.Sprintf("content:%s:%s:%d", algorithm, fullHash, event.FileSize)
	contentExists := false
	for _, content := range m.contents {
		if content.ContentID == contentID {
			contentExists = true
			break
		}
	}
	if !contentExists {
		m.contents = append(m.contents, roe.Content{
			ContentID: contentID, HashAlgorithm: algorithm, FullHash: fullHash, FileSize: event.FileSize,
		})
	}
	assetID := fmt.Sprintf("asset:%d:%s", event.OwnerID, contentID)
	assetExists := false
	for _, asset := range m.assets {
		if asset.AssetID == assetID {
			assetExists = true
			break
		}
	}
	if !assetExists {
		m.assets = append(m.assets, roe.Asset{AssetID: assetID, OwnerID: event.OwnerID, ContentID: contentID})
		m.outbox = append(m.outbox, roe.Effect{
			Key: "process:" + assetID, AssetID: assetID, ContentID: contentID, ExpectedRevision: event.ExpectedRevision,
		})
	}
	for _, location := range m.locations {
		if location.NodeID == event.NodeID && location.UnboundRevision == 0 && location.AssetID == assetID && location.BoundRevision == event.ExpectedRevision {
			return roe.Result{Code: roe.ResultNoop, Reason: "binding already active"}
		}
	}
	m.closeLocation(event.NodeID, event.ExpectedRevision)
	m.locations = append(m.locations, roe.Location{
		LocationID: fmt.Sprintf("location:%s:%d", event.NodeID, event.ExpectedRevision),
		NodeID:     event.NodeID, AssetID: assetID, BoundRevision: event.ExpectedRevision,
	})
	return roe.Result{Code: roe.ResultApplied}
}

func (m *Model) finalize(event roe.Event) roe.Result {
	directory, ok := m.node(event.NodeID)
	if !ok || !directory.Active || directory.Kind != roe.NodeDirectory {
		return roe.Result{Code: roe.ResultDeferred, Reason: "covered directory is not active"}
	}
	if !event.Authoritative || event.CoverageError || !event.CursorHealthy {
		if event.CoverageError || !event.CursorHealthy {
			m.fullVerificationRequired = true
		}
		return roe.Result{Code: roe.ResultNoop, Reason: "coverage cannot finalize absence"}
	}
	for index, node := range m.nodes {
		if !node.Active || node.ParentID != event.NodeID || nodeListed(event.ObservedChildren, node.NodeID) {
			continue
		}
		m.nodes[index].Active = false
		if event.Revision > m.nodes[index].Revision {
			m.nodes[index].Revision = event.Revision
		}
		m.closeLocation(node.NodeID, event.Revision)
	}
	m.coverage[event.NodeID] = event.Revision
	if event.Revision >= m.cursorRevision {
		m.cursorRevision = event.Revision
	}
	m.fullVerificationRequired = false
	return roe.Result{Code: roe.ResultApplied}
}

func (m *Model) node(id roe.NodeID) (roe.Node, bool) {
	for _, node := range m.nodes {
		if node.NodeID == id {
			return node, true
		}
	}
	return roe.Node{}, false
}

func (m *Model) putNode(node roe.Node) {
	for index := range m.nodes {
		if m.nodes[index].NodeID == node.NodeID {
			m.nodes[index] = node
			return
		}
	}
	m.nodes = append(m.nodes, node)
}

func (m *Model) closeLocation(nodeID roe.NodeID, revision uint64) {
	for index := range m.locations {
		if m.locations[index].NodeID == nodeID && m.locations[index].UnboundRevision == 0 {
			m.locations[index].UnboundRevision = revision
		}
	}
}

func (m *Model) Snapshot() roe.Snapshot {
	snapshot := roe.Snapshot{
		NextSequence:             m.nextSequence,
		CursorRevision:           m.cursorRevision,
		FullVerificationRequired: m.fullVerificationRequired,
		Nodes:                    append([]roe.Node(nil), m.nodes...),
		Contents:                 append([]roe.Content(nil), m.contents...),
		Assets:                   append([]roe.Asset(nil), m.assets...),
		Locations:                append([]roe.Location(nil), m.locations...),
		Outbox:                   append([]roe.Effect(nil), m.outbox...),
		AppliedKeys:              append([]string(nil), m.appliedKeys...),
		Coverage:                 make(map[roe.NodeID]uint64, len(m.coverage)),
	}
	for _, location := range m.locations {
		if location.UnboundRevision == 0 {
			snapshot.ActiveLocations = append(snapshot.ActiveLocations, location)
		}
	}
	for _, event := range m.pending {
		snapshot.Pending = append(snapshot.Pending, copyEvent(event))
	}
	for nodeID, revision := range m.coverage {
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

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func nodeListed(values []roe.NodeID, target roe.NodeID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func copyEvent(event roe.Event) roe.Event {
	event.ObservedChildren = append([]roe.NodeID(nil), event.ObservedChildren...)
	return event
}
