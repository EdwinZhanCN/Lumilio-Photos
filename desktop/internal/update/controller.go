package update

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

// ErrSignature marks an update the provider refused because it is not signed
// with the key this build trusts. A provider wraps it, so the controller can
// tell a refused artifact from a failed download.
var ErrSignature = errors.New("update signature verification failed")

// Provider is the signed-update transport. It finds a newer release on a
// channel, verifies what it downloads against the public key built into the
// app, and replaces the installed app. The host implements it with the
// framework's updater; nothing here knows where releases are published.
type Provider interface {
	// Enabled reports whether this build can update itself: it was built
	// with an update feed and can write where it is installed.
	Enabled() bool
	// Check returns the newer release of channel, or nil when the app is up
	// to date.
	Check(ctx context.Context, channel string) (*Release, error)
}

// Release is a newer version a Provider found.
type Release struct {
	Version string
	// Install downloads the release, verifies its signature, and replaces
	// the app on disk. The running app is not affected: the new version
	// runs after a relaunch. progress may be called from any goroutine.
	Install func(ctx context.Context, progress func(downloaded, total int64)) error
}

type Options struct {
	Store      *state.Store
	Operations *operation.Registry
	Provider   Provider
	// Apply quiesces the application and relaunches it into the installed
	// update. It belongs to the shutdown coordinator.
	Apply          func(requestID string, expectedVersion uint64) (dto.OperationReceipt, error)
	Channel        string
	CurrentVersion string
}

// Controller owns the update aggregate: its snapshot, its operations, and
// the order check, download, apply. Trust is the Provider's; the controller
// never installs what a Provider did not verify.
type Controller struct {
	store          *state.Store
	operations     *operation.Registry
	provider       Provider
	apply          func(string, uint64) (dto.OperationReceipt, error)
	channel        string
	currentVersion string
	release        *Release
	installed      bool
	mu             sync.Mutex
}

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
		provider: options.Provider, apply: options.Apply,
		channel: options.Channel, currentVersion: options.CurrentVersion,
	}
	controller.commit(func(snapshot *dto.UpdateSnapshot) {
		if snapshot.Version == 0 {
			snapshot.Version = 1
		}
		snapshot.Phase = "idle"
	})
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
	// A release that is already installed on disk stays ready to apply: the
	// channel only decides what the next check looks for.
	if !c.installed {
		c.release = nil
	}
	installed := c.installed
	c.mu.Unlock()
	if installed {
		c.commit(func(update *dto.UpdateSnapshot) { update.Version++ })
		return nil
	}
	c.commit(func(update *dto.UpdateSnapshot) {
		update.Phase = "idle"
		update.AvailableVersion = ""
		update.Progress = 0
		update.Error = dto.Error{}
		update.CanApply = false
		update.Version++
	})
	return nil
}

// SetProvider installs the transport once the host has one.
func (c *Controller) SetProvider(provider Provider) {
	c.mu.Lock()
	c.provider = provider
	c.mu.Unlock()
	c.commit(func(*dto.UpdateSnapshot) {})
}

func (c *Controller) SetApply(callback func(string, uint64) (dto.OperationReceipt, error)) {
	c.mu.Lock()
	c.apply = callback
	canApply := callback != nil && c.installed
	c.mu.Unlock()
	c.commit(func(update *dto.UpdateSnapshot) { update.CanApply = canApply })
}

func (c *Controller) enabledProvider() Provider {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.provider == nil || !c.provider.Enabled() {
		return nil
	}
	return c.provider
}

func (c *Controller) Check(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.start(requestID, expectedVersion, "checking", func(ctx context.Context) (dto.UpdateSnapshot, error) {
		release, result, err := c.find(ctx)
		if release == nil {
			return result, err
		}
		return dto.UpdateSnapshot{Phase: "available", AvailableVersion: release.Version}, nil
	})
}

// find asks the provider for the channel's newer release and remembers it.
// Without one it returns the snapshot that says why.
func (c *Controller) find(ctx context.Context) (*Release, dto.UpdateSnapshot, error) {
	provider := c.enabledProvider()
	if provider == nil {
		return nil, dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update provider is not configured"}}, nil
	}
	c.mu.Lock()
	channel := c.channel
	c.mu.Unlock()
	release, err := provider.Check(ctx, channel)
	switch {
	case errors.Is(err, ErrSignature):
		return nil, dto.UpdateSnapshot{Phase: "failed", Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update signature verification failed"}}, err
	case err != nil:
		return nil, dto.UpdateSnapshot{Phase: "offline", Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update check failed"}}, nil
	case release == nil:
		c.mu.Lock()
		c.release = nil
		c.mu.Unlock()
		return nil, dto.UpdateSnapshot{Phase: "current"}, nil
	}
	c.mu.Lock()
	c.release = release
	c.mu.Unlock()
	return release, dto.UpdateSnapshot{}, nil
}

// progressInterval bounds how often a download republishes the snapshot.
const progressInterval = 200 * time.Millisecond

func (c *Controller) Download(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return c.start(requestID, expectedVersion, "downloading", func(ctx context.Context) (dto.UpdateSnapshot, error) {
		c.mu.Lock()
		release := c.release
		c.mu.Unlock()
		if release == nil {
			found, result, err := c.find(ctx)
			if found == nil {
				return result, err
			}
			release = found
		}
		c.commit(func(update *dto.UpdateSnapshot) { update.AvailableVersion = release.Version })
		var last time.Time
		var progressMu sync.Mutex
		err := release.Install(ctx, func(downloaded, total int64) {
			if total <= 0 {
				return
			}
			progressMu.Lock()
			due := time.Since(last) >= progressInterval
			if due {
				last = time.Now()
			}
			progressMu.Unlock()
			if due {
				c.commit(func(update *dto.UpdateSnapshot) { update.Progress = float64(downloaded) / float64(total) })
			}
		})
		switch {
		case errors.Is(err, ErrSignature):
			return dto.UpdateSnapshot{Phase: "failed", AvailableVersion: release.Version, Error: dto.Error{Code: dto.ErrorSignatureInvalid, Message: "update signature verification failed"}}, err
		case err != nil:
			return dto.UpdateSnapshot{Phase: "offline", AvailableVersion: release.Version, Error: dto.Error{Code: dto.ErrorRuntimeNotReady, Message: "update download failed"}}, nil
		}
		c.mu.Lock()
		c.installed = true
		canApply := c.apply != nil
		c.mu.Unlock()
		return dto.UpdateSnapshot{Phase: "ready", AvailableVersion: release.Version, CanApply: canApply}, nil
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
	installed := c.installed
	c.mu.Unlock()
	if apply == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorShutdownInProgress, "update apply requires the shutdown coordinator")
	}
	if !snapshot.Update.CanApply || !installed {
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
	c.commit(func(update *dto.UpdateSnapshot) {
		update.Phase = phase
		update.Progress = 0
		update.Error = dto.Error{}
		update.Version++
	})
	go func() {
		result, workErr := work(context.Background())
		// Publish the result before the operation turns terminal, so anyone
		// who observes the finished operation also observes its snapshot.
		c.commit(func(update *dto.UpdateSnapshot) {
			version := update.Version
			*update = result
			update.Version = version + 1
		})
		if workErr != nil {
			failure := workErr
			if result.Error.Code != "" {
				failure = operation.NewError(result.Error.Code, result.Error.Message)
			}
			_ = c.operations.Fail(receipt.OperationID, operation.WithOperation(failure, receipt.OperationID))
		} else {
			_ = c.operations.Succeed(receipt.OperationID)
		}
		c.syncOperations()
	}()
	return receipt, nil
}

func (c *Controller) commit(update func(*dto.UpdateSnapshot)) {
	c.mu.Lock()
	channel := c.channel
	currentVersion := c.currentVersion
	providerAvailable := c.provider != nil && c.provider.Enabled()
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
