package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()

	validCases := map[string]string{
		"alex":       "alex",
		"Alex.Dev":   "alex.dev",
		"alex_dev":   "alex_dev",
		"alex-2026":  "alex-2026",
		"  Lumilio ": "lumilio",
	}

	for input, expected := range validCases {
		actual, err := normalizeUsername(input)
		require.NoError(t, err)
		require.Equal(t, expected, actual)
	}

	invalidCases := []string{
		"ab",
		"1alex",
		".alex",
		"alex-",
		"alex__dev",
		"alex..dev",
		"alex-_dev",
		"alex dev",
		"用户",
	}

	for _, input := range invalidCases {
		_, err := normalizeUsername(input)
		require.ErrorIs(t, err, ErrInvalidUsernameFormat)
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	t.Parallel()

	validCases := []string{
		"",
		"Alex",
		"张子豪",
		"山田 太郎",
		"Привет",
	}

	for _, input := range validCases {
		_, err := normalizeDisplayName(input)
		require.NoError(t, err)
	}

	_, err := normalizeDisplayName("hello\nworld")
	require.ErrorIs(t, err, ErrInvalidDisplayName)
}

func TestValidatePasswordPolicy(t *testing.T) {
	t.Parallel()

	require.NoError(t, validatePasswordPolicy("Lumilio2026"))
	require.NoError(t, validatePasswordPolicy("PasskeyFlow9"))

	invalidCases := []string{
		"short9A",
		"alllowercase2026",
		"ALLUPPERCASE2026",
		"NoDigitsHere",
	}

	for _, input := range invalidCases {
		require.ErrorIs(t, validatePasswordPolicy(input), ErrWeakPassword)
	}
}
