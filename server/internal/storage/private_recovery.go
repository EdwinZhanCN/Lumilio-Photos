package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RepositoryRecoveryDirectory holds the private-recovery receipts of one
// repository and, under <receipt_id>/, the staging bytes a receipt moved out
// of the way. The receipt is portable: a catalog that never saw the
// repository reads it back and presents the same inventory.
const RepositoryRecoveryDirectory = ".lumilio/recovery"

// RecoveryReceiptFormat is the receipt format this build writes and reads.
// Later builds keep reading format 1; this build reports a newer format
// instead of guessing at it.
const RecoveryReceiptFormat = 1

// ErrRecoveryReceiptFormat reports a receipt this build cannot read.
var ErrRecoveryReceiptFormat = errors.New("unsupported private-recovery receipt")

// recoveryItemLimit bounds the per-file records a receipt carries. Trash,
// logs, and derived artifacts are summarized; Studio, staging, and unknown
// files are listed because each needs a decision.
const recoveryItemLimit = 2000

// PrivateStateClass names one kind of repository-private data under
// .lumilio/. Preservation is decided per class, never for the tree as a whole.
type PrivateStateClass string

const (
	// PrivateClassTrash is .lumilio/trash: files Delete moved there and their
	// format-1 info sidecars. They stay in place; the Trash rebuild adopts them
	// into the catalog under a hold.
	PrivateClassTrash PrivateStateClass = "trash"
	// PrivateClassStudio is a .lumilio/sidecars/*.lumilio-sidecar Studio edit
	// record. It stays in place and is reassociated only on proven content.
	PrivateClassStudio PrivateStateClass = "studio"
	// PrivateClassStaging is an uncommitted upload or import under
	// .lumilio/staging. It is moved under the receipt so ordinary staging
	// cleanup cannot discard it, and never resumed automatically.
	PrivateClassStaging PrivateStateClass = "staging"
	// PrivateClassLogs is .lumilio/logs: diagnostic logs, kept in place.
	PrivateClassLogs PrivateStateClass = "logs"
	// PrivateClassDerived is .lumilio/assets and .lumilio/artifacts:
	// rebuildable derivatives, kept in place and rederived as needed.
	PrivateClassDerived PrivateStateClass = "derived"
	// PrivateClassUnknown is anything else, including content a newer build
	// wrote. It is preserved unchanged and disclosed.
	PrivateClassUnknown PrivateStateClass = "unknown"
)

// RecoveryEvent is the registration that produced a receipt.
type RecoveryEvent string

const (
	RecoveryEventOpen         RecoveryEvent = "open"
	RecoveryEventRegisterCopy RecoveryEvent = "register_copy"
	RecoveryEventAdopt        RecoveryEvent = "adopt"
)

// RecoveryDisposition is what became of one recorded item.
type RecoveryDisposition string

const (
	// DispositionPreserved: the bytes are where the receipt found them.
	DispositionPreserved RecoveryDisposition = "preserved"
	// DispositionMoved: the bytes were moved under the receipt directory.
	DispositionMoved RecoveryDisposition = "moved"
	// DispositionReassociated: a Studio record now carries its new Asset ID.
	DispositionReassociated RecoveryDisposition = "reassociated"
	// DispositionAmbiguous: a Studio record's source could not be proven;
	// it is kept for review or export.
	DispositionAmbiguous RecoveryDisposition = "ambiguous"
	// DispositionUnreadable: a record in a format this build does not read.
	DispositionUnreadable RecoveryDisposition = "unreadable"
	// DispositionDeleted: the user deleted the item from the inventory.
	DispositionDeleted RecoveryDisposition = "deleted"
)

// RecoverySummary counts one class that is preserved in place as a whole.
type RecoverySummary struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

// RecoveryItem is one file the receipt lists for a decision.
type RecoveryItem struct {
	ItemID      string              `json:"item_id"`
	Class       PrivateStateClass   `json:"class"`
	Path        string              `json:"path"`
	Size        int64               `json:"size"`
	MtimeNs     int64               `json:"mtime_ns"`
	Disposition RecoveryDisposition `json:"disposition"`
	// MovedTo is the current private path of a moved staging file.
	MovedTo string `json:"moved_to,omitempty"`
	// AssetID is the Asset a Studio record named when it was written;
	// ResolvedAssetID is the Asset it was reassociated to.
	AssetID         string     `json:"asset_id,omitempty"`
	ResolvedAssetID string     `json:"resolved_asset_id,omitempty"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
	// Detail is a short reason for an unreadable or ambiguous item.
	Detail string `json:"detail,omitempty"`
}

// CurrentPath is where the item's bytes live now.
func (item RecoveryItem) CurrentPath() string {
	if item.MovedTo != "" {
		return item.MovedTo
	}
	return item.Path
}

// RecoveryReceipt records what a registration found under .lumilio/ and what
// it did about it. Format 1.
type RecoveryReceipt struct {
	Format     int           `json:"format"`
	ReceiptID  string        `json:"receipt_id"`
	Event      RecoveryEvent `json:"event"`
	RecordedAt time.Time     `json:"recorded_at"`
	Actor      string        `json:"actor,omitempty"`
	RequestID  string        `json:"request_id,omitempty"`
	// SourceRepositoryID is the marker identity the tree carried when it was
	// registered; RepositoryID is the identity it was registered under. They
	// differ for an independent copy.
	SourceRepositoryID string          `json:"source_repository_id"`
	RepositoryID       string          `json:"repository_id"`
	Trash              RecoverySummary `json:"trash"`
	Logs               RecoverySummary `json:"logs"`
	Derived            RecoverySummary `json:"derived"`
	Items              []RecoveryItem  `json:"items"`
	// ItemsTruncated counts files beyond the receipt's item limit; they are
	// preserved in place but not listed individually.
	ItemsTruncated int `json:"items_truncated,omitempty"`
}

// Marshal encodes a format-1 receipt.
func (r RecoveryReceipt) Marshal() ([]byte, error) {
	r.Format = RecoveryReceiptFormat
	if err := r.validate(); err != nil {
		return nil, err
	}
	if r.Items == nil {
		r.Items = []RecoveryItem{}
	}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

// ParseRecoveryReceipt reads a receipt. It accepts format 1, including
// fields a later build may add, and rejects any other format.
func ParseRecoveryReceipt(data []byte) (RecoveryReceipt, error) {
	var header struct {
		Format *int `json:"format"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return RecoveryReceipt{}, fmt.Errorf("%w: %v", ErrRecoveryReceiptFormat, err)
	}
	if header.Format == nil {
		return RecoveryReceipt{}, fmt.Errorf("%w: no format field", ErrRecoveryReceiptFormat)
	}
	if *header.Format != RecoveryReceiptFormat {
		return RecoveryReceipt{}, fmt.Errorf("%w: format %d, this build reads format %d", ErrRecoveryReceiptFormat, *header.Format, RecoveryReceiptFormat)
	}
	var receipt RecoveryReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return RecoveryReceipt{}, fmt.Errorf("%w: %v", ErrRecoveryReceiptFormat, err)
	}
	if err := receipt.validate(); err != nil {
		return RecoveryReceipt{}, err
	}
	return receipt, nil
}

func (r RecoveryReceipt) validate() error {
	if _, err := uuid.Parse(r.ReceiptID); err != nil {
		return fmt.Errorf("%w: receipt_id is not a UUID", ErrRecoveryReceiptFormat)
	}
	switch r.Event {
	case RecoveryEventOpen, RecoveryEventRegisterCopy, RecoveryEventAdopt:
	default:
		return fmt.Errorf("%w: unknown event %q", ErrRecoveryReceiptFormat, r.Event)
	}
	if r.RecordedAt.IsZero() {
		return fmt.Errorf("%w: missing recorded_at", ErrRecoveryReceiptFormat)
	}
	for _, item := range r.Items {
		if _, err := ParsePrivateRepositoryPath(item.Path); err != nil {
			return fmt.Errorf("%w: item path %q: %v", ErrRecoveryReceiptFormat, item.Path, err)
		}
		if item.MovedTo != "" {
			if _, err := ParsePrivateRepositoryPath(item.MovedTo); err != nil {
				return fmt.Errorf("%w: moved path %q: %v", ErrRecoveryReceiptFormat, item.MovedTo, err)
			}
		}
	}
	return nil
}

// RecoveryReceiptPath is the receipt file of one registration.
func RecoveryReceiptPath(receiptID string) (RepositoryPath, error) {
	if _, err := uuid.Parse(receiptID); err != nil {
		return RepositoryPath{}, fmt.Errorf("%w: receipt id %q", ErrRepositoryPathInvalid, receiptID)
	}
	return ParsePrivateRepositoryPath(path.Join(RepositoryRecoveryDirectory, receiptID+".json"))
}

// RecoveryReceiptDirectory holds the bytes one receipt moved.
func RecoveryReceiptDirectory(receiptID string) (RepositoryPath, error) {
	if _, err := uuid.Parse(receiptID); err != nil {
		return RepositoryPath{}, fmt.Errorf("%w: receipt id %q", ErrRepositoryPathInvalid, receiptID)
	}
	return ParsePrivateRepositoryPath(path.Join(RepositoryRecoveryDirectory, receiptID))
}

// recoveryItemID derives a stable item identity from the receipt and the
// path the receipt recorded, so a retry or a second catalog names the same item.
func recoveryItemID(receiptID, itemPath string) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("lumilio-recovery-item/"+receiptID+"/"+itemPath)).String()
}

// ListRecoveryReceipts reads every receipt of an opened repository, oldest
// first. A receipt this build cannot read is returned by name in unreadable.
func ListRecoveryReceipts(ctx context.Context, files *RepositoryFS) (receipts []RecoveryReceipt, unreadable []string, err error) {
	directory, err := ParsePrivateRepositoryPath(RepositoryRecoveryDirectory)
	if err != nil {
		return nil, nil, err
	}
	exists, err := files.Exists(directory)
	if err != nil || !exists {
		return nil, nil, err
	}
	root, done, err := files.withRoot()
	if err != nil {
		return nil, nil, err
	}
	entries, readErr := fs.ReadDir(root.FS(), directory.String())
	done()
	if readErr != nil {
		return nil, nil, fmt.Errorf("read recovery receipts: %w", readErr)
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		receiptPath, err := ParsePrivateRepositoryPath(path.Join(directory.String(), entry.Name()))
		if err != nil {
			unreadable = append(unreadable, entry.Name())
			continue
		}
		data, err := files.ReadPrivateFile(receiptPath)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := ParseRecoveryReceipt(data)
		if err != nil || receipt.ReceiptID+".json" != entry.Name() {
			unreadable = append(unreadable, entry.Name())
			continue
		}
		receipts = append(receipts, receipt)
	}
	sort.Slice(receipts, func(i, j int) bool {
		if !receipts[i].RecordedAt.Equal(receipts[j].RecordedAt) {
			return receipts[i].RecordedAt.Before(receipts[j].RecordedAt)
		}
		return receipts[i].ReceiptID < receipts[j].ReceiptID
	})
	return receipts, unreadable, nil
}

// WriteRecoveryReceipt replaces one receipt atomically. Callers hold the
// repository's work lease; the receipt is the durable record of the items'
// dispositions, so it is written after the disk change it describes.
func WriteRecoveryReceipt(files *RepositoryFS, receipt RecoveryReceipt) error {
	target, err := RecoveryReceiptPath(receipt.ReceiptID)
	if err != nil {
		return err
	}
	data, err := receipt.Marshal()
	if err != nil {
		return err
	}
	directory, err := ParsePrivateRepositoryPath(RepositoryRecoveryDirectory)
	if err != nil {
		return err
	}
	if err := files.MkdirAllPrivate(directory, 0o755); err != nil {
		return err
	}
	_, err = files.WritePrivateFileAtomic(target, bytes.NewReader(data), 0o644)
	return err
}

// privateStateMove is one staging file the receipt moves out of
// .lumilio/staging. From and To are private repository paths.
type privateStateMove struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// privatePreservationPlan is what a registration journals before it touches
// the private tree: the receipt it will write and the moves it will make.
// Rollback and crash recovery replay it in reverse.
type privatePreservationPlan struct {
	Receipt *RecoveryReceipt   `json:"receipt,omitempty"`
	Moves   []privateStateMove `json:"moves,omitempty"`
	// PrivateRootAbsent records that the tree had no .lumilio/ at all, so a
	// rollback removes the fresh layout the registration created.
	PrivateRootAbsent bool `json:"private_root_absent,omitempty"`
}

// Empty reports a plan with nothing to record: a fresh tree, or one with
// no private state at all.
func (plan privatePreservationPlan) Empty() bool { return plan.Receipt == nil }

// rollbackPrivateState undoes a registration's private-state changes: the
// preservation plan in reverse, and the fresh layout when the tree had none.
func rollbackPrivateState(repositoryPath string, plan privatePreservationPlan) error {
	if plan.PrivateRootAbsent {
		privateRoot := path.Join(repositoryPath, DefaultStructure.SystemDir)
		if err := os.RemoveAll(privateRoot); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove fresh repository private state: %w", err)
		}
		return nil
	}
	return rollbackPrivatePreservation(repositoryPath, plan)
}

// ReceiptID is the plan's receipt identity, or "".
func (plan privatePreservationPlan) ReceiptID() string {
	if plan.Receipt == nil {
		return ""
	}
	return plan.Receipt.ReceiptID
}

// Test seams for the preservation fault-injection proofs. They run only
// when a test installs them.
var (
	testHookAfterRecoveryReceiptWritten func() error
	testHookAfterPrivateStateMove       func(moved int) error
)

type privateWalkEntry struct {
	relative string // path below .lumilio, slash separated
	size     int64
	mtimeNs  int64
	regular  bool
}

// planPrivatePreservation inventories .lumilio/ of an unregistered tree and
// plans the class-specific preservation. Nothing is written. Trash, Studio,
// logs, and derivatives stay in place; staging is planned to move under the
// receipt directory; unknown and newer content is listed and left alone.
// Receipts and bytes of earlier registrations are recognized and skipped.
func planPrivatePreservation(repositoryPath string, event RecoveryEvent, actor, requestID, sourceRepositoryID, repositoryID string, now time.Time) (privatePreservationPlan, error) {
	privateRoot := path.Join(repositoryPath, DefaultStructure.SystemDir)
	root, err := os.OpenRoot(privateRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return privatePreservationPlan{PrivateRootAbsent: true}, nil
	}
	if err != nil {
		return privatePreservationPlan{}, fmt.Errorf("read repository private state: %w", err)
	}
	defer root.Close()

	receiptID := uuid.NewString()
	receipt := RecoveryReceipt{
		Format: RecoveryReceiptFormat, ReceiptID: receiptID, Event: event, RecordedAt: now.UTC(),
		Actor: actor, RequestID: requestID, SourceRepositoryID: sourceRepositoryID, RepositoryID: repositoryID,
		Items: []RecoveryItem{},
	}
	var plan privatePreservationPlan
	knownReceiptDirs := map[string]bool{}
	topLevel, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return privatePreservationPlan{}, fmt.Errorf("read repository private state: %w", err)
	}
	if len(topLevel) == 0 {
		return privatePreservationPlan{}, nil
	}
	for _, entry := range topLevel {
		if entry.Name() == "recovery" && entry.IsDir() {
			children, err := fs.ReadDir(root.FS(), "recovery")
			if err != nil {
				return privatePreservationPlan{}, err
			}
			for _, child := range children {
				if !child.IsDir() && strings.HasSuffix(child.Name(), ".json") {
					knownReceiptDirs[strings.TrimSuffix(child.Name(), ".json")] = true
				}
			}
		}
	}

	addItem := func(item RecoveryItem) {
		if len(receipt.Items) >= recoveryItemLimit {
			receipt.ItemsTruncated++
			return
		}
		item.ItemID = recoveryItemID(receiptID, item.Path)
		receipt.Items = append(receipt.Items, item)
	}
	walkErr := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		first, rest, _ := strings.Cut(name, "/")
		privatePath := path.Join(DefaultStructure.SystemDir, name)
		if entry.IsDir() {
			// Receipts and the bytes earlier registrations moved are theirs.
			if first == "recovery" && rest != "" && !strings.Contains(rest, "/") && knownReceiptDirs[rest] {
				return fs.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			addItem(RecoveryItem{Class: PrivateClassUnknown, Path: privatePath, Disposition: DispositionPreserved, Detail: "not a regular file"})
			return nil
		}
		item := RecoveryItem{Path: privatePath, Size: info.Size(), MtimeNs: info.ModTime().UnixNano(), Disposition: DispositionPreserved}
		switch first {
		case "trash":
			if strings.HasPrefix(rest, "info/") || strings.HasPrefix(rest, "files/") {
				receipt.Trash.Files++
				receipt.Trash.Bytes += info.Size()
				return nil
			}
			item.Class = PrivateClassUnknown
			addItem(item)
		case "sidecars":
			if strings.Contains(rest, "/") || !strings.HasSuffix(rest, ".lumilio-sidecar") {
				item.Class = PrivateClassUnknown
				addItem(item)
				return nil
			}
			item.Class = PrivateClassStudio
			data, err := root.ReadFile(name)
			if err != nil {
				return err
			}
			header, headerErr := readStudioSidecarHeader(data)
			switch {
			case headerErr != nil:
				item.Disposition = DispositionUnreadable
				item.Detail = headerErr.Error()
			default:
				item.AssetID = header.AssetID
			}
			addItem(item)
		case "staging":
			if _, err := ParsePrivateRepositoryPath(privatePath); err != nil {
				item.Class = PrivateClassUnknown
				item.Detail = "path is not portable"
				addItem(item)
				return nil
			}
			destination := path.Join(RepositoryRecoveryDirectory, receiptID, name)
			if _, err := ParsePrivateRepositoryPath(destination); err != nil {
				item.Class = PrivateClassUnknown
				item.Detail = "path is not portable"
				addItem(item)
				return nil
			}
			item.Class = PrivateClassStaging
			item.Disposition = DispositionMoved
			item.MovedTo = destination
			plan.Moves = append(plan.Moves, privateStateMove{From: privatePath, To: destination})
			addItem(item)
		case "logs":
			receipt.Logs.Files++
			receipt.Logs.Bytes += info.Size()
		case "assets", "artifacts":
			receipt.Derived.Files++
			receipt.Derived.Bytes += info.Size()
		case "recovery":
			if !strings.Contains(rest, "/") && strings.HasSuffix(rest, ".json") && knownReceiptDirs[strings.TrimSuffix(rest, ".json")] {
				return nil
			}
			item.Class = PrivateClassUnknown
			item.Detail = "left by an earlier build"
			addItem(item)
		default:
			item.Class = PrivateClassUnknown
			addItem(item)
		}
		return nil
	})
	if walkErr != nil {
		return privatePreservationPlan{}, fmt.Errorf("inventory repository private state: %w", walkErr)
	}
	plan.Receipt = &receipt
	return plan, nil
}

// studioSidecarHeader is the part of a Studio record preservation reads:
// enough to know it is a version this build understands and which Asset it
// named. Full decoding stays with the Studio DTO.
type studioSidecarHeader struct {
	Version int    `json:"version"`
	AssetID string `json:"asset_id"`
}

// StudioSidecarVersion is the Studio record version this build reads.
const StudioSidecarVersion = 1

func readStudioSidecarHeader(data []byte) (studioSidecarHeader, error) {
	var header struct {
		Version *int   `json:"version"`
		AssetID string `json:"asset_id"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return studioSidecarHeader{}, fmt.Errorf("not a Studio record: %v", err)
	}
	if header.Version == nil || *header.Version != StudioSidecarVersion {
		version := 0
		if header.Version != nil {
			version = *header.Version
		}
		return studioSidecarHeader{}, fmt.Errorf("Studio record version %d; this build reads version %d", version, StudioSidecarVersion)
	}
	if _, err := uuid.Parse(header.AssetID); err != nil {
		return studioSidecarHeader{}, errors.New("Studio record names no Asset")
	}
	return studioSidecarHeader{Version: *header.Version, AssetID: header.AssetID}, nil
}

// applyPrivatePreservation writes the receipt, then moves staging under it.
// Every step is idempotent, so a retry after a crash finishes the plan: a
// receipt already present is kept, a file already at its destination counts
// as moved, and a file found under both names (a move that stopped between
// link and unlink) keeps the destination.
func applyPrivatePreservation(repositoryPath string, plan privatePreservationPlan) error {
	if plan.Empty() {
		return nil
	}
	privateRoot := path.Join(repositoryPath, DefaultStructure.SystemDir)
	root, err := os.OpenRoot(privateRoot)
	if err != nil {
		return fmt.Errorf("open repository private state: %w", err)
	}
	defer root.Close()
	recoveryDir := path.Join("recovery", plan.ReceiptID())
	if err := root.MkdirAll("recovery", 0o755); err != nil {
		return fmt.Errorf("create recovery directory: %w", err)
	}
	receiptName := path.Join("recovery", plan.ReceiptID()+".json")
	if _, err := root.Lstat(receiptName); errors.Is(err, fs.ErrNotExist) {
		data, err := plan.Receipt.Marshal()
		if err != nil {
			return err
		}
		if err := writeRootFileAtomic(root, receiptName, data, 0o644); err != nil {
			return fmt.Errorf("write recovery receipt: %w", err)
		}
	} else if err != nil {
		return err
	}
	if testHookAfterRecoveryReceiptWritten != nil {
		if err := testHookAfterRecoveryReceiptWritten(); err != nil {
			return err
		}
	}
	if len(plan.Moves) > 0 {
		if err := root.MkdirAll(recoveryDir, 0o700); err != nil {
			return fmt.Errorf("create recovery staging directory: %w", err)
		}
	}
	for index, move := range plan.Moves {
		from, to, err := privateMoveNames(move.From, move.To)
		if err != nil {
			return err
		}
		if err := root.MkdirAll(path.Dir(to), 0o700); err != nil {
			return fmt.Errorf("create recovery staging directory: %w", err)
		}
		if err := moveRootFileNoReplace(root, from, to); err != nil {
			return fmt.Errorf("preserve staging %s: %w", move.From, err)
		}
		if testHookAfterPrivateStateMove != nil {
			if err := testHookAfterPrivateStateMove(index + 1); err != nil {
				return err
			}
		}
	}
	return nil
}

// rollbackPrivatePreservation undoes a plan: staging goes back where the
// receipt found it and the receipt is removed. It tolerates a plan that was
// applied partially or not at all, and leaves a file alone when it is already
// back at its source.
func rollbackPrivatePreservation(repositoryPath string, plan privatePreservationPlan) error {
	if plan.Empty() {
		return nil
	}
	privateRoot := path.Join(repositoryPath, DefaultStructure.SystemDir)
	root, err := os.OpenRoot(privateRoot)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open repository private state: %w", err)
	}
	defer root.Close()
	var failures []error
	for index := len(plan.Moves) - 1; index >= 0; index-- {
		move := plan.Moves[index]
		from, to, err := privateMoveNames(move.From, move.To)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if _, err := root.Lstat(to); errors.Is(err, fs.ErrNotExist) {
			if _, sourceErr := root.Lstat(from); sourceErr == nil {
				continue // never moved
			}
			failures = append(failures, fmt.Errorf("staging %s is missing from both locations", move.From))
			continue
		}
		if err := root.MkdirAll(path.Dir(from), 0o700); err != nil {
			failures = append(failures, err)
			continue
		}
		if err := moveRootFileNoReplace(root, to, from); err != nil {
			failures = append(failures, fmt.Errorf("restore staging %s: %w", move.From, err))
		}
	}
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	receiptName := path.Join("recovery", plan.ReceiptID()+".json")
	if err := root.Remove(receiptName); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove recovery receipt: %w", err)
	}
	removeEmptyRootDirectories(root, path.Join("recovery", plan.ReceiptID()))
	_ = root.Remove("recovery")
	return nil
}

func privateMoveNames(from, to string) (string, string, error) {
	fromPath, err := ParsePrivateRepositoryPath(from)
	if err != nil {
		return "", "", err
	}
	toPath, err := ParsePrivateRepositoryPath(to)
	if err != nil {
		return "", "", err
	}
	prefix := DefaultStructure.SystemDir + "/"
	if !strings.HasPrefix(fromPath.String(), prefix) || !strings.HasPrefix(toPath.String(), prefix) {
		return "", "", fmt.Errorf("%w: move outside the private tree", ErrRepositoryPathNamespace)
	}
	return strings.TrimPrefix(fromPath.String(), prefix), strings.TrimPrefix(toPath.String(), prefix), nil
}

// moveRootFileNoReplace moves one regular file inside a root without ever
// replacing an existing destination. Link-then-unlink is atomic about the
// destination where hard links exist; a destination that already is the same
// file completes an interrupted move; a different file there is a conflict.
func moveRootFileNoReplace(root *os.Root, from, to string) error {
	sourceInfo, sourceErr := root.Lstat(from)
	destinationInfo, destinationErr := root.Lstat(to)
	switch {
	case destinationErr == nil && sourceErr == nil:
		if os.SameFile(sourceInfo, destinationInfo) {
			return root.Remove(from)
		}
		return fmt.Errorf("%w: %s", ErrRepositoryDestinationExists, to)
	case destinationErr == nil && errors.Is(sourceErr, fs.ErrNotExist):
		return nil // already moved
	case sourceErr != nil:
		return sourceErr
	case !errors.Is(destinationErr, fs.ErrNotExist):
		return destinationErr
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s is not a regular file", ErrRepositoryEntryUnsupported, from)
	}
	if err := root.Link(from, to); err == nil {
		if err := syncRootFile(root, to); err != nil {
			_ = root.Remove(to)
			return err
		}
		return root.Remove(from)
	} else if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%w: %s", ErrRepositoryDestinationExists, to)
	}
	// No hard links on this volume: rename after the existence check above.
	return root.Rename(from, to)
}

func syncRootFile(root *os.Root, name string) error {
	file, err := root.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	return errors.Join(err, file.Close())
}

// writeRootFileAtomic publishes a complete file through a same-directory
// temporary name; a crash leaves no partial destination.
func writeRootFileAtomic(root *os.Root, name string, data []byte, perm fs.FileMode) error {
	temporary := name + ".tmp-" + uuid.NewString()
	file, err := root.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = file.Close()
			_ = root.Remove(temporary)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := root.Rename(temporary, name); err != nil {
		return err
	}
	cleanup = false
	if directory, err := root.Open(path.Dir(name)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

// removeEmptyRootDirectories removes a directory tree that holds no files,
// deepest first; anything non-empty is left alone.
func removeEmptyRootDirectories(root *os.Root, name string) {
	entries, err := fs.ReadDir(root.FS(), name)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			removeEmptyRootDirectories(root, path.Join(name, entry.Name()))
		}
	}
	_ = root.Remove(name)
}
