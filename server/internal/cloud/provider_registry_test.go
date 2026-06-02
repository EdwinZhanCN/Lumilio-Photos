package cloud

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultProviderRegistry_OnlyEnablesICloud(t *testing.T) {
	registry := NewDefaultProviderRegistry()

	providers := registry.List()
	require.Len(t, providers, 1)

	descriptor := providers[0].Descriptor()
	require.Equal(t, ProviderICloud, descriptor.ID)
	require.Equal(t, ProviderStatusEnabled, descriptor.Status)
	require.NotEmpty(t, descriptor.FormFields)
	require.NotEmpty(t, descriptor.ChallengeFields)
}

func TestProviderRegistry_RejectsUnknownProvider(t *testing.T) {
	registry := NewDefaultProviderRegistry()

	_, err := registry.Get(ProviderKind("webdav"))
	require.ErrorContains(t, err, "unsupported cloud provider")
}
