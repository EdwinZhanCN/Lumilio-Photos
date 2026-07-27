package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"desktop/supervisor"

	"github.com/stretchr/testify/require"
)

func TestParseDesktopCLI(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		breakGlass bool
		username   string
		wantErr    string
	}{
		{name: "normal launch"},
		{name: "default admin", args: []string{"--break-glass"}, breakGlass: true},
		{name: "explicit admin", args: []string{"--break-glass", "--break-glass-username", " Admin "}, breakGlass: true, username: "Admin"},
		{name: "username without recovery", args: []string{"--break-glass-username", "admin"}, wantErr: "--break-glass-username requires --break-glass"},
		{name: "positional argument", args: []string{"unexpected"}, wantErr: "unexpected positional arguments"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			controls, err := parseDesktopCLI(test.args, io.Discard)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.breakGlass, controls.BreakGlass)
			require.Equal(t, test.username, controls.BreakGlassUsername)
		})
	}
}

func TestOnlyHostLockConflictQuitsAfterRuntimeStartFailure(t *testing.T) {
	require.True(t, runtimeStartFailureIsHostFatal(supervisor.ErrAlreadyRunning))
	require.True(t, runtimeStartFailureIsHostFatal(fmt.Errorf("wrapped: %w", supervisor.ErrAlreadyRunning)))
	require.False(t, runtimeStartFailureIsHostFatal(supervisor.ErrPortInUse))
	require.False(t, runtimeStartFailureIsHostFatal(errors.New("strict manifest rejected")))
}

func TestDesktopHostLocksBeforePersistedStateOrUI(t *testing.T) {
	appData := filepath.Join(t.TempDir(), "appdata")
	t.Setenv("LUMILIO_APP_DATA", appData)

	first := newDesktopApp()
	t.Cleanup(first.cancel)
	_, err := os.Stat(appData)
	require.ErrorIs(t, err, os.ErrNotExist, "constructor must not touch persisted state before host lock")

	require.NoError(t, first.prepareHost())
	second := newDesktopApp()
	t.Cleanup(second.cancel)
	require.ErrorIs(t, second.prepareHost(), supervisor.ErrAlreadyRunning)

	require.NoError(t, first.sup.Close())
	require.NoError(t, second.prepareHost(), "lock should be available after the first host closes")
	require.NoError(t, second.sup.Close())
}

func TestParseDesktopCLIDoesNotReadStandaloneEnvironment(t *testing.T) {
	t.Setenv("LUMILIO_BREAK_GLASS", "true")
	t.Setenv("LUMILIO_BREAK_GLASS_USERNAME", "admin")

	controls, err := parseDesktopCLI(nil, io.Discard)
	require.NoError(t, err)
	require.False(t, controls.BreakGlass)
	require.Empty(t, controls.BreakGlassUsername)
}
