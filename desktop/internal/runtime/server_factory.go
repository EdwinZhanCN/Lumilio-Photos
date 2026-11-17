package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"server/app"
	"server/config"
)

// AppConfigLoader supplies one already-strict-loaded manifest for each new
// generation. The factory never searches for config or applies environment
// overrides.
type AppConfigLoader func() (config.AppConfig, error)

type ServerFactory struct {
	Load AppConfigLoader
}

func (f ServerFactory) Start(parent context.Context, id uint64) (Generation, error) {
	if f.Load == nil {
		return Generation{}, errors.New("server config loader is unavailable")
	}
	cfg, err := f.Load()
	if err != nil {
		return Generation{}, classifyGenerationFailure(err)
	}
	manifestFingerprint := cfg.ManifestSHA256
	if !strings.HasPrefix(manifestFingerprint, "sha256:") {
		manifestFingerprint = "sha256:" + manifestFingerprint
	}
	ctx, cancel := context.WithCancel(parent)
	done := make(chan error, 1)
	ready := make(chan ReadyInfo, 1)
	var mu sync.Mutex
	var runtimeReady *app.RuntimeInfo
	var repositoryControl app.RepositoryControl
	var published bool
	publish := func() {
		mu.Lock()
		defer mu.Unlock()
		if published || runtimeReady == nil || repositoryControl == nil {
			return
		}
		published = true
		ready <- ReadyInfo{
			Runtime:           *runtimeReady,
			RepositoryControl: repositoryControl,
			ManifestSHA256:    manifestFingerprint,
		}
	}
	controls := app.OperatorControls{
		RuntimeReady: func(info app.RuntimeInfo) {
			mu.Lock()
			copy := info
			runtimeReady = &copy
			mu.Unlock()
			publish()
		},
		RepositoryManagerReady: func(control app.RepositoryControl) {
			if control == nil {
				return
			}
			mu.Lock()
			repositoryControl = control
			mu.Unlock()
			publish()
		},
	}
	go func() {
		done <- classifyGenerationFailure(app.Run(ctx, cfg, controls))
	}()
	return Generation{
		ID: id, Cancel: cancel, Done: done, Ready: ready,
		ManifestSHA256: manifestFingerprint,
	}, nil
}

// classifyGenerationFailure turns a classified Server startup failure into
// the control-plane error the snapshot's RecoveryCause and the operation
// receipt carry, with the Server's own guidance as the message. Any other
// error is returned unchanged, so ordinary lifecycle outcomes keep their
// meaning.
func classifyGenerationFailure(err error) error {
	if err == nil {
		return nil
	}
	failure := app.ClassifyStartupFailure(err)
	var code dto.ErrorCode
	switch failure.Code {
	case app.StartupFailureCatalogLineageUnsupported, app.StartupFailureCatalogNewer:
		code = dto.ErrorCatalogIncompatible
	case app.StartupFailureConfigurationIncompatible, app.StartupFailureConfigurationUpgradeRequired:
		code = dto.ErrorConfigurationIncompatible
	case app.StartupFailureDefaultStorageMigrationRejected:
		code = dto.ErrorStorageMigrationRejected
	case app.StartupFailureDefaultStorageUnavailable:
		code = dto.ErrorStorageUnavailable
	default:
		return err
	}
	return operation.NewError(code, fmt.Sprintf("%v. %s", failure.Cause, strings.Join(failure.Guidance(), " ")))
}
