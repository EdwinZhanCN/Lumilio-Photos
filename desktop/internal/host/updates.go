package host

import (
	"context"
	"fmt"
	"strings"

	"desktop/internal/update"

	"github.com/egoist/mygo"
)

// frameworkUpdates is the update transport: the framework's updater, which
// reads the signed manifests `mygo build` publishes, verifies each archive
// and delta against the public key linked into this build, and replaces the
// installed app. A build made without an update feed reports Enabled false.
type frameworkUpdates struct{}

func (frameworkUpdates) Enabled() bool { return mygo.Updater.Enabled() }

func (frameworkUpdates) Check(ctx context.Context, channel string) (*update.Release, error) {
	// The Beta channel is the releases published as prereleases; Stable sees
	// only the latest full release.
	mygo.Updater.SetPrereleases(channel == "beta")
	found, err := mygo.Updater.Check(ctx)
	if err != nil {
		return nil, classifyUpdateError(err)
	}
	if found == nil {
		return nil, nil
	}
	return &update.Release{
		Version: found.Version,
		Install: func(ctx context.Context, progress func(downloaded, total int64)) error {
			return classifyUpdateError(found.Install(ctx, progress))
		},
	}, nil
}

// classifyUpdateError marks the updater's refusal of an artifact that is not
// signed with this build's key. The updater exposes no typed error for it,
// so its message is the contract; an unrecognized message stays an ordinary
// failure, which is retried rather than reported as tampering.
func classifyUpdateError(err error) error {
	if err != nil && strings.Contains(err.Error(), "not signed with the app's key") {
		return fmt.Errorf("%w: %w", update.ErrSignature, err)
	}
	return err
}
