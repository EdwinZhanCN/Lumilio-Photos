package service

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	usernameMinLength    = 3
	usernameMaxLength    = 32
	displayNameMaxLength = 64
	passwordMinLength    = 10
	passwordMaxLength    = 72
)

var (
	ErrInvalidUsernameFormat = errors.New("username must be 3-32 characters, start with a letter, use lowercase letters, numbers, '.', '_' or '-', and may not end with or repeat separators")
	ErrInvalidDisplayName    = errors.New("display name must be 64 characters or fewer and cannot contain control characters")
	ErrWeakPassword          = errors.New("password must be 10-72 characters and include at least one lowercase letter, one uppercase letter, and one number")
)

func normalizeUsername(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	length := utf8.RuneCountInString(normalized)
	if length < usernameMinLength || length > usernameMaxLength {
		return "", ErrInvalidUsernameFormat
	}

	previousWasSeparator := false
	for index, r := range normalized {
		isSeparator := r == '.' || r == '_' || r == '-'
		isAlpha := r >= 'a' && r <= 'z'
		isNumeric := r >= '0' && r <= '9'

		switch {
		case index == 0 && !isAlpha:
			return "", ErrInvalidUsernameFormat
		case !(isAlpha || isNumeric || isSeparator):
			return "", ErrInvalidUsernameFormat
		case isSeparator && (previousWasSeparator || index == len(normalized)-1):
			return "", ErrInvalidUsernameFormat
		}

		previousWasSeparator = isSeparator
	}

	return normalized, nil
}

func normalizeDisplayName(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	if utf8.RuneCountInString(trimmed) > displayNameMaxLength {
		return "", ErrInvalidDisplayName
	}

	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return "", ErrInvalidDisplayName
		}
	}

	return trimmed, nil
}

func validatePasswordPolicy(password string) error {
	if len(password) < passwordMinLength || len(password) > passwordMaxLength {
		return ErrWeakPassword
	}

	var hasLower, hasUpper, hasDigit bool
	for _, r := range password {
		switch {
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}

	if !hasLower || !hasUpper || !hasDigit {
		return ErrWeakPassword
	}

	return nil
}
