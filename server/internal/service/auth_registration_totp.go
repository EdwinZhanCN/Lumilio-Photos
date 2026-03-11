package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"server/internal/db/repo"

	"github.com/jackc/pgx/v5"
)

type RegistrationTOTPSetupResponse struct {
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
	OtpAuthURI  string `json:"otpauth_uri"`
}

type RegistrationTOTPCompleteResponse struct {
	AuthResponse  *AuthResponse `json:"auth"`
	RecoveryCodes []string      `json:"recovery_codes"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

func (s *AuthService) BeginRegistrationTOTPSetup(ctx context.Context, registrationSessionID string) (RegistrationTOTPSetupResponse, error) {
	session, err := s.getActiveRegistrationSession(ctx, registrationSessionID)
	if err != nil {
		return RegistrationTOTPSetupResponse{}, err
	}

	secret := ""
	if len(session.TotpSecretCiphertext) > 0 {
		if existingSecret, err := s.decryptMFASecret(session.TotpSecretCiphertext); err == nil {
			secret = existingSecret
		}
	}

	if secret == "" {
		secret, err = generateTOTPSecret()
		if err != nil {
			return RegistrationTOTPSetupResponse{}, err
		}

		encryptedSecret, err := s.encryptMFASecret(secret)
		if err != nil {
			return RegistrationTOTPSetupResponse{}, fmt.Errorf("encrypt registration totp secret: %w", err)
		}

		if _, err := s.queries.UpdateRegistrationSessionTOTPSecret(ctx, repo.UpdateRegistrationSessionTOTPSecretParams{
			SessionID:            session.SessionID,
			TotpSecretCiphertext: encryptedSecret,
		}); err != nil {
			return RegistrationTOTPSetupResponse{}, fmt.Errorf("persist registration totp secret: %w", err)
		}
	}

	accountName := session.Username

	return RegistrationTOTPSetupResponse{
		Secret:      secret,
		Issuer:      defaultTOTPIssuer,
		AccountName: accountName,
		OtpAuthURI:  buildTOTPAuthURI(defaultTOTPIssuer, accountName, secret),
	}, nil
}

func (s *AuthService) CompleteRegistrationTOTP(ctx context.Context, registrationSessionID string, code string) (RegistrationTOTPCompleteResponse, error) {
	session, err := s.getActiveRegistrationSession(ctx, registrationSessionID)
	if err != nil {
		return RegistrationTOTPCompleteResponse{}, err
	}

	if len(session.TotpSecretCiphertext) == 0 {
		return RegistrationTOTPCompleteResponse{}, ErrRegistrationTOTPNotPrepared
	}

	secret, err := s.decryptMFASecret(session.TotpSecretCiphertext)
	if err != nil {
		return RegistrationTOTPCompleteResponse{}, fmt.Errorf("decrypt registration totp secret: %w", err)
	}

	if !validateTOTPCode(secret, code, time.Now()) {
		return RegistrationTOTPCompleteResponse{}, ErrInvalidMFACode
	}

	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return RegistrationTOTPCompleteResponse{}, err
	}

	user, role, err := s.finalizeRegistrationWithTOTP(ctx, session, recoveryHashes)
	if err != nil {
		return RegistrationTOTPCompleteResponse{}, err
	}

	authResponse, err := s.generateAuthResponse(user)
	if err != nil {
		return RegistrationTOTPCompleteResponse{}, err
	}
	authResponse.BootstrapAdmin = role == UserRoleAdmin

	return RegistrationTOTPCompleteResponse{
		AuthResponse:  authResponse,
		RecoveryCodes: recoveryCodes,
		GeneratedAt:   time.Now(),
	}, nil
}

func (s *AuthService) finalizeRegistrationWithTOTP(ctx context.Context, session repo.RegistrationSession, recoveryHashes []string) (repo.User, UserRole, error) {
	var (
		createdUser repo.User
		finalRole   UserRole
	)

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if _, err := q.GetUserByUsername(ctx, session.Username); err == nil {
			return ErrUserAlreadyExists
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check username availability: %w", err)
		}

		role, err := determineRegistrationRole(ctx, q, session.Role)
		if err != nil {
			return err
		}
		finalRole = role

		user, err := q.CreateUser(ctx, repo.CreateUserParams{
			Username:           session.Username,
			Password:           session.PasswordHash,
			DisplayName:        session.Username,
			Role:               string(role),
			WebauthnUserHandle: session.WebauthnUserHandle,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}

		if _, err := q.UpsertUserTOTPCredential(ctx, repo.UpsertUserTOTPCredentialParams{
			UserID:           user.UserID,
			SecretCiphertext: session.TotpSecretCiphertext,
		}); err != nil {
			return fmt.Errorf("create totp credential: %w", err)
		}

		for _, hash := range recoveryHashes {
			if err := q.CreateUserRecoveryCode(ctx, repo.CreateUserRecoveryCodeParams{
				UserID:   user.UserID,
				CodeHash: hash,
			}); err != nil {
				return fmt.Errorf("create recovery code: %w", err)
			}
		}

		if err := q.DeleteRegistrationSession(ctx, session.SessionID); err != nil {
			return fmt.Errorf("delete registration session: %w", err)
		}

		createdUser = user
		return nil
	}); err != nil {
		return repo.User{}, UserRoleUser, err
	}

	return createdUser, finalRole, nil
}
