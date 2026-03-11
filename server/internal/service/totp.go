package service

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base32"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

const (
	totpDigits          = 6
	totpPeriod          = 30 * time.Second
	totpAllowedTimeSkew = 1

	recoveryCodeCount  = 10
	recoveryCodeLength = 8
)

const recoveryCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

var totpBase32Encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func generateTOTPSecret() (string, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}

	return totpBase32Encoding.EncodeToString(secret), nil
}

func validateTOTPCode(secret string, code string, now time.Time) bool {
	normalizedCode := normalizeTOTPCode(code)
	if len(normalizedCode) != totpDigits {
		return false
	}

	for offset := -totpAllowedTimeSkew; offset <= totpAllowedTimeSkew; offset++ {
		codeAtTime, err := generateTOTPCode(secret, now.Add(time.Duration(offset)*totpPeriod))
		if err == nil && subtleEqual(codeAtTime, normalizedCode) {
			return true
		}
	}

	return false
}

func generateTOTPCode(secret string, now time.Time) (string, error) {
	normalizedSecret := strings.ToUpper(strings.TrimSpace(secret))
	decodedSecret, err := totpBase32Encoding.DecodeString(normalizedSecret)
	if err != nil {
		return "", fmt.Errorf("decode totp secret: %w", err)
	}

	counter := uint64(now.Unix() / int64(totpPeriod/time.Second))
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	mac := hmac.New(sha1.New, decodedSecret)
	if _, err := mac.Write(counterBytes); err != nil {
		return "", fmt.Errorf("write totp counter: %w", err)
	}

	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	binaryCode := (int(sum[offset])&0x7f)<<24 |
		(int(sum[offset+1])&0xff)<<16 |
		(int(sum[offset+2])&0xff)<<8 |
		(int(sum[offset+3]) & 0xff)

	return fmt.Sprintf("%0*d", totpDigits, binaryCode%1000000), nil
}

func generateRecoveryCodes() ([]string, []string, error) {
	codes := make([]string, 0, recoveryCodeCount)
	hashes := make([]string, 0, recoveryCodeCount)

	for len(codes) < recoveryCodeCount {
		raw := make([]byte, recoveryCodeLength)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, fmt.Errorf("generate recovery code: %w", err)
		}

		var builder strings.Builder
		builder.Grow(recoveryCodeLength)
		for _, b := range raw {
			builder.WriteByte(recoveryCodeAlphabet[int(b)%len(recoveryCodeAlphabet)])
		}

		normalized := builder.String()
		formatted := normalized[:4] + "-" + normalized[4:]
		hash := hashRecoveryCode(normalized)

		duplicate := false
		for _, existingHash := range hashes {
			if existingHash == hash {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}

		codes = append(codes, formatted)
		hashes = append(hashes, hash)
	}

	return codes, hashes, nil
}

func hashRecoveryCode(code string) string {
	normalized := normalizeRecoveryCode(code)
	if normalized == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func normalizeRecoveryCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")

	var builder strings.Builder
	builder.Grow(len(normalized))
	for _, char := range normalized {
		if (char >= 'A' && char <= 'Z') || (char >= '2' && char <= '9') {
			builder.WriteRune(char)
		}
	}

	return builder.String()
}

func normalizeTOTPCode(code string) string {
	normalized := strings.TrimSpace(code)
	normalized = strings.ReplaceAll(normalized, " ", "")

	var builder strings.Builder
	builder.Grow(len(normalized))
	for _, char := range normalized {
		if char >= '0' && char <= '9' {
			builder.WriteRune(char)
		}
	}

	return builder.String()
}

func subtleEqual(left string, right string) bool {
	return hmac.Equal([]byte(left), []byte(right))
}
