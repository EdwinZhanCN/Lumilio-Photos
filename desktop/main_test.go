package main

import (
	"io"
	"testing"

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

func TestParseDesktopCLIDoesNotReadStandaloneEnvironment(t *testing.T) {
	t.Setenv("LUMILIO_BREAK_GLASS", "true")
	t.Setenv("LUMILIO_BREAK_GLASS_USERNAME", "admin")

	controls, err := parseDesktopCLI(nil, io.Discard)
	require.NoError(t, err)
	require.False(t, controls.BreakGlass)
	require.Empty(t, controls.BreakGlassUsername)
}
