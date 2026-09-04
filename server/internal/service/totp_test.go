package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidateTOTPCode(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	at := time.Unix(0, 0).UTC()

	code, err := generateTOTPCode(secret, at)
	require.NoError(t, err)
	require.Equal(t, "282760", code)
	require.True(t, validateTOTPCode(secret, code, at))
	require.True(t, validateTOTPCode(secret, code, at.Add(25*time.Second)))
	require.False(t, validateTOTPCode(secret, "000000", at))
}

func TestRecoveryCodeGenerationAndNormalization(t *testing.T) {
	codes, hashes, err := generateRecoveryCodes()
	require.NoError(t, err)
	require.Len(t, codes, recoveryCodeCount)
	require.Len(t, hashes, recoveryCodeCount)

	first := codes[0]
	require.Len(t, normalizeRecoveryCode(first), recoveryCodeLength)
	require.Equal(t, hashRecoveryCode(first), hashRecoveryCode(normalizeRecoveryCode(first)))
}

func TestNormalizeMFAMethodRejectsUnknownValues(t *testing.T) {
	if got := normalizeMFAMethod("bogus"); got != "" {
		t.Fatalf("normalizeMFAMethod(bogus) = %q, want empty", got)
	}
	if got := normalizeMFAMethod(MFAMethodTOTP); got != MFAMethodTOTP {
		t.Fatalf("normalizeMFAMethod(totp) = %q, want %q", got, MFAMethodTOTP)
	}
	if got := normalizeMFAMethod(MFAMethodRecoveryCode); got != MFAMethodRecoveryCode {
		t.Fatalf("normalizeMFAMethod(recovery_code) = %q, want %q", got, MFAMethodRecoveryCode)
	}
}
