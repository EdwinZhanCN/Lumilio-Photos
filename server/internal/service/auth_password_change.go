package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
)

const (
	passwordChangeTokenTTL     = 10 * time.Minute
	passwordChangeTokenPurpose = "required_password_change"
)

type passwordChangeClaims struct {
	UserID      int    `json:"user_id"`
	AuthVersion int64  `json:"auth_version"`
	Purpose     string `json:"purpose"`
	jwt.RegisteredClaims
}

func (s *AuthService) issueRequiredPasswordChange(user repo.User) (*AuthResponse, error) {
	now := time.Now()
	claims := passwordChangeClaims{
		UserID:      int(user.UserID),
		AuthVersion: user.AuthVersion,
		Purpose:     passwordChangeTokenPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(passwordChangeTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    "lumilio-photos",
			Subject:   strconv.Itoa(int(user.UserID)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.passwordChangeTokenSecret)
	if err != nil {
		return nil, fmt.Errorf("sign password change token: %w", err)
	}
	return &AuthResponse{
		User:                   ptr(ConvertUserToResponse(user)),
		RequiresPasswordChange: true,
		PasswordChangeToken:    token,
	}, nil
}

func (s *AuthService) parsePasswordChangeToken(tokenString string) (*passwordChangeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &passwordChangeClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidPasswordChangeToken
		}
		return s.passwordChangeTokenSecret, nil
	})
	if err != nil {
		return nil, ErrInvalidPasswordChangeToken
	}
	claims, ok := token.Claims.(*passwordChangeClaims)
	if !ok || !token.Valid || claims.Purpose != passwordChangeTokenPurpose || claims.UserID <= 0 {
		return nil, ErrInvalidPasswordChangeToken
	}
	return claims, nil
}

// CompleteRequiredPasswordChange consumes a short-lived, single-purpose token.
// The conditional update increments auth_version, making concurrent or replayed
// uses of the same token fail closed.
func (s *AuthService) CompleteRequiredPasswordChange(ctx context.Context, tokenString, newPassword string) (*AuthResponse, error) {
	claims, err := s.parsePasswordChangeToken(tokenString)
	if err != nil {
		return nil, err
	}
	passwordHash, err := bcryptPassword(newPassword)
	if err != nil {
		return nil, err
	}

	var updated repo.User
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		user, err := q.CompleteRequiredPasswordChange(ctx, repo.CompleteRequiredPasswordChangeParams{
			UserID:      int32(claims.UserID),
			AuthVersion: claims.AuthVersion,
			Password:    passwordHash,
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidPasswordChangeToken
			}
			return fmt.Errorf("complete required password change: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, user.UserID); err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}
		updated = user
		return nil
	}); err != nil {
		return nil, err
	}

	return s.generateAuthResponse(updated)
}
