package update

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/state"
)

type Fetcher func(context.Context, string) (manifestBytes []byte, artifact []byte, err error)

type Options struct {
	Store          *state.Store
	Operations     *operation.Registry
	PublicKey      ed25519.PublicKey
	Fetch          Fetcher
	Apply          func(string, uint64) (dto.OperationReceipt, error)
	StagingDir     string
	Channel        string
	CurrentVersion string
}

type Controller struct {
	store          *state.Store
	operations     *operation.Registry
	publicKey      ed25519.PublicKey
	fetch          Fetcher
	apply          func(string, uint64) (dto.OperationReceipt, error)
	stagingDir     string
	channel        string
	currentVersion string
	verified       *verifiedArtifact
	mu             sync.Mutex
}

type verifiedArtifact struct {
	manifestBytes []byte
	manifest      Manifest
	artifact      []byte
}

type stagingPointer struct {
	SchemaVersion int    `json:"schemaVersion"`
	Channel       string `json:"channel"`
	Version       string `json:"version"`
	SHA256        string `json:"sha256"`
	Directory     string `json:"directory"`
}

const stagingSchemaVersion = 1

func NewController(options Options) *Controller {
	if options.Store == nil {
		options.Store = state.New()
	}
	if options.Operations == nil {
		options.Operations = operation.New()
	}
	if strings.TrimSpace(options.Channel) == "" {
		options.Channel = "stable"
	}
	controller := &Controller{
		store: options.Store, operations: options.Operations,
		publicKey: append(ed25519.PublicKey(nil), options.PublicKey...), fetch: options.Fetch, apply: options.Apply,
		stagingDir: options.StagingDir, channel: options.Channel, currentVersion: options.CurrentVersion,
	}
	controller.commit(func(snapshot *dto.UpdateSnapshot) {
		if snapshot.Version == 0 {
			snapshot.Version = 1
		}
		snapshot.Phase = "idle"
	})
	if staged, err := controller.loadStaged(); err == nil && staged.manifest.Channel == controller.channel {
		controller.verified = staged
		controller.commit(func(snapshot *dto.UpdateSnapshot) {
			snapshot.Phase = "ready"
			snapshot.AvailableVersion = staged.manifest.Version
			snapshot.CanApply = options.Apply != nil
		})
	}
	return controller
}

func (c *Controller) Snapshot() dto.UpdateSnapshot { return c.store.Get().Update }

func (c *Controller) SetChannel(channel string) error {
	channel = strings.TrimSpace(channel)
	if channel != "stable" && channel != "beta" {
		return operation.NewError(dto.ErrorInvalidArgument, "update channel must be stable or beta")
	}
	c.mu.Lock()
	if c.channel == channel {
		c.mu.Unlock()
		return nil
	}
	c.channel = channel
	c.verified = nil
	c.mu.Unlock()
	c.commit(func(update *dto.UpdateSnapshot) {
		update.Phase = "idle"
		update.AvailableVersion = ""
		update.Error = dto.Error{}
		update.CanApply = false
		update.Version++
	})
	return nil
}

func (c *Controller) SetApply(callback func(string, uint64) (dto.OperationReceipt, error)) {
	c.mu.Lock()
	c.apply = callback
	canApply := callback != nil && c.verified != nil
	c.mu.Unlock()
	c.commit(func(update *dto.UpdateSnapshot) { update.CanApply = canApply })
}

func (c *Controller) Check(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.start(requestID, expectedVersion, "checking", func(ctx context.Context) (dto.UpdateSnapshot, error) {
		if c.fetch == nil {
			return dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update provider is not configured"}}, nil
		}
		data, artifact, err := c.fetch(ctx, c.store.Get().Update.Channel)
		if err != nil {
			return dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update check failed"}}, nil
		}
		manifest, err := VerifyManifest(data, c.publicKey)
		if err != nil {
			return dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update signature verification failed"}}, err
		}
		if err := VerifyArtifact(manifest, artifact); err != nil {
			return dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update artifact verification failed"}}, err
		}
		c.mu.Lock()
		c.verified = &verifiedArtifact{manifestBytes: append([]byte(nil), data...), manifest: manifest, artifact: append([]byte(nil), artifact...)}
		c.mu.Unlock()
		return dto.UpdateSnapshot{Phase: "available", Channel: manifest.Channel, AvailableVersion: manifest.Version}, nil
	})
}

func (c *Controller) Download(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.start(requestID, expectedVersion, "downloading", func(ctx context.Context) (dto.UpdateSnapshot, error) {
		c.mu.Lock()
		verified := c.verified
		fetch := c.fetch
		channel := c.store.Get().Update.Channel
		c.mu.Unlock()
		if verified == nil {
			if fetch == nil {
				return dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update provider is not configured"}}, nil
			}
			manifestBytes, artifact, err := fetch(ctx, channel)
			if err != nil {
				return dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update download failed"}}, nil
			}
			manifest, err := VerifyManifest(manifestBytes, c.publicKey)
			if err != nil {
				return dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update signature verification failed"}}, err
			}
			if err := VerifyArtifact(manifest, artifact); err != nil {
				return dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update artifact verification failed"}}, err
			}
			verified = &verifiedArtifact{manifestBytes: append([]byte(nil), manifestBytes...), manifest: manifest, artifact: append([]byte(nil), artifact...)}
			c.mu.Lock()
			c.verified = verified
			c.mu.Unlock()
		}
		if err := c.stage(verified); err != nil {
			return dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorRecoveryRequired, Message: "verified update could not be staged"}}, err
		}
		c.mu.Lock()
		canApply := c.apply != nil
		c.mu.Unlock()
		return dto.UpdateSnapshot{Phase: "ready", Channel: verified.manifest.Channel, AvailableVersion: verified.manifest.Version, CanApply: canApply}, nil
	})
}

func (c *Controller) RestartAndApply(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Update.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "update snapshot version is stale")
	}
	c.mu.Lock()
	apply := c.apply
	verified := c.verified
	c.mu.Unlock()
	if apply == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorShutdownInProgress, "update apply requires the shutdown coordinator")
	}
	if !snapshot.Update.CanApply || verified == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "no verified update is ready to apply")
	}
	receipt, err := c.operations.Accept(requestID, "update", snapshot.Update.Version, false)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	_ = c.operations.MarkRunning(receipt.OperationID)
	c.commit(func(update *dto.UpdateSnapshot) { update.Phase = "applying"; update.Version++ })
	if _, err := apply(requestID, snapshot.Update.Version); err != nil {
		wrapped := operation.WithOperation(err, receipt.OperationID)
		_ = c.operations.Fail(receipt.OperationID, wrapped)
		c.commit(func(update *dto.UpdateSnapshot) {
			update.Phase = "failed"
			update.Error = dto.Error{Code: operation.ErrorCodeOf(err), Message: "update shutdown handoff failed", OperationID: receipt.OperationID}
			update.CanApply = true
			update.Version++
		})
		c.syncOperations()
		return dto.OperationReceipt{}, wrapped
	}
	_ = c.operations.Succeed(receipt.OperationID)
	c.syncOperations()
	return receipt, nil
}

func (c *Controller) stage(verified *verifiedArtifact) error {
	if verified == nil {
		return errors.New("no verified update artifact is available")
	}
	if strings.TrimSpace(c.stagingDir) == "" {
		return errors.New("update staging directory is not configured")
	}
	if err := platform.EnsurePrivateDirectory(c.stagingDir); err != nil {
		return err
	}
	directory := safeSegment(verified.manifest.Version) + "-" + strings.TrimPrefix(verified.manifest.SHA256, "sha256:")[:12]
	target := filepath.Join(c.stagingDir, directory)
	if info, err := os.Stat(target); err == nil {
		if !info.IsDir() {
			return errors.New("update staging target is not a directory")
		}
		if err := verifyStagedDirectory(target, verified, c.publicKey); err != nil {
			return err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	} else {
		staging, err := os.MkdirTemp(c.stagingDir, ".staging-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		if err := platform.WriteAtomic(filepath.Join(staging, "manifest.json"), verified.manifestBytes, 0o600); err != nil {
			return err
		}
		if err := platform.WriteAtomic(filepath.Join(staging, "artifact.bin"), verified.artifact, 0o700); err != nil {
			return err
		}
		if err := os.Rename(staging, target); err != nil {
			return err
		}
	}
	pointer := stagingPointer{SchemaVersion: stagingSchemaVersion, Channel: verified.manifest.Channel, Version: verified.manifest.Version, SHA256: verified.manifest.SHA256, Directory: directory}
	data, err := json.MarshalIndent(pointer, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(filepath.Join(c.stagingDir, "current.json"), append(data, '\n'), 0o600)
}

func verifyStagedDirectory(directory string, verified *verifiedArtifact, publicKey ed25519.PublicKey) error {
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return err
	}
	manifest, err := VerifyManifest(manifestBytes, publicKey)
	if err != nil {
		return err
	}
	artifact, err := os.ReadFile(filepath.Join(directory, "artifact.bin"))
	if err != nil {
		return err
	}
	if manifest.Channel != verified.manifest.Channel || manifest.Version != verified.manifest.Version || manifest.SHA256 != verified.manifest.SHA256 {
		return errors.New("existing update staging target does not match the verified manifest")
	}
	return VerifyArtifact(manifest, artifact)
}

func (c *Controller) loadStaged() (*verifiedArtifact, error) {
	if strings.TrimSpace(c.stagingDir) == "" {
		return nil, errors.New("update staging directory is not configured")
	}
	data, err := os.ReadFile(filepath.Join(c.stagingDir, "current.json"))
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var pointer stagingPointer
	if err := decoder.Decode(&pointer); err != nil {
		return nil, err
	}
	if pointer.SchemaVersion != stagingSchemaVersion || pointer.Channel == "" || pointer.Version == "" || pointer.SHA256 == "" {
		return nil, errors.New("invalid update staging pointer")
	}
	directory, err := safeJoin(c.stagingDir, pointer.Directory)
	if err != nil {
		return nil, err
	}
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "manifest.json"))
	if err != nil {
		return nil, err
	}
	manifest, err := VerifyManifest(manifestBytes, c.publicKey)
	if err != nil {
		return nil, err
	}
	artifact, err := os.ReadFile(filepath.Join(directory, "artifact.bin"))
	if err != nil {
		return nil, err
	}
	if manifest.Channel != pointer.Channel || manifest.Version != pointer.Version || manifest.SHA256 != pointer.SHA256 {
		return nil, errors.New("update staging pointer does not match its signed manifest")
	}
	if err := VerifyArtifact(manifest, artifact); err != nil {
		return nil, err
	}
	return &verifiedArtifact{manifestBytes: manifestBytes, manifest: manifest, artifact: artifact}, nil
}

func safeJoin(root, relative string) (string, error) {
	if relative == "" || filepath.IsAbs(relative) || filepath.Base(relative) != relative || relative == "." || relative == ".." {
		return "", fmt.Errorf("invalid update staging directory %q", relative)
	}
	joined := filepath.Clean(filepath.Join(root, relative))
	cleanRoot := filepath.Clean(root)
	if !strings.HasPrefix(joined, cleanRoot+string(os.PathSeparator)) {
		return "", errors.New("update staging path escapes its root")
	}
	return joined, nil
}

func safeSegment(value string) string {
	var result strings.Builder
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			result.WriteRune(character)
		} else {
			result.WriteByte('_')
		}
	}
	if result.Len() == 0 {
		return "unknown"
	}
	return result.String()
}

func (c *Controller) start(requestID string, expectedVersion uint64, phase string, work func(context.Context) (dto.UpdateSnapshot, error)) (dto.OperationReceipt, error) {
	if existing, ok := c.operations.ReceiptForRequest(requestID); ok {
		return existing, nil
	}
	snapshot := c.store.Get()
	if expectedVersion != 0 && expectedVersion != snapshot.Update.Version {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorStaleVersion, "update snapshot version is stale")
	}
	receipt, err := c.operations.Accept(requestID, "update", snapshot.Update.Version, true)
	if err != nil {
		return dto.OperationReceipt{}, err
	}
	_ = c.operations.MarkRunning(receipt.OperationID)
	c.commit(func(update *dto.UpdateSnapshot) { update.Phase = phase; update.Version++ })
	go func() {
		result, workErr := work(context.Background())
		if workErr != nil {
			failure := workErr
			if result.Error.Code != "" {
				failure = operation.NewError(result.Error.Code, result.Error.Message)
			}
			_ = c.operations.Fail(receipt.OperationID, operation.WithOperation(failure, receipt.OperationID))
		} else {
			_ = c.operations.Succeed(receipt.OperationID)
		}
		c.commit(func(update *dto.UpdateSnapshot) {
			*update = result
			update.Version++
		})
		c.syncOperations()
	}()
	return receipt, nil
}

func (c *Controller) commit(update func(*dto.UpdateSnapshot)) {
	c.mu.Lock()
	channel := c.channel
	currentVersion := c.currentVersion
	providerAvailable := c.fetch != nil
	c.mu.Unlock()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		update(&snapshot.Update)
		snapshot.Update.Channel = channel
		snapshot.Update.CurrentVersion = currentVersion
		snapshot.Update.ProviderAvailable = providerAvailable
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func (c *Controller) syncOperations() {
	items := c.operations.Snapshot()
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Operations = items })
}

var _ interface {
	Snapshot() dto.UpdateSnapshot
	Check(string, uint64) (dto.OperationReceipt, error)
	Download(string, uint64) (dto.OperationReceipt, error)
	RestartAndApply(string, uint64) (dto.OperationReceipt, error)
	SetChannel(string) error
} = (*Controller)(nil)
