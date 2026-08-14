package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultTOTPIssuer       = "Lumilio Photos"
	loginMFAChallengeTTL    = 5 * time.Minute
	totpSetupTokenTTL       = 10 * time.Minute
	securityVerificationTTL = 5 * time.Minute
	mfaTokenPurposeLogin    = "mfa_login"
)

const (
	MFAMethodTOTP         = "totp"
	MFAMethodRecoveryCode = "recovery_code"
)

var (
	ErrInvalidMFAToken      = errors.New("invalid mfa token")
	ErrExpiredMFAToken      = errors.New("expired mfa token")
	ErrInvalidMFACode       = errors.New("invalid mfa code")
	ErrMFANotEnabled        = errors.New("mfa is not enabled")
	ErrInvalidCurrentSecret = errors.New("current password is incorrect")
	ErrInvalidSecurityProof = errors.New("invalid or expired security verification")
	ErrInvalidMFAState      = errors.New("inconsistent mfa state")
)

type MFAStatus struct {
	TOTPEnabled              bool       `json:"totp_enabled"`
	PasskeyCount             int        `json:"passkey_count"`
	RecoveryCodesRemaining   int        `json:"recovery_codes_remaining"`
	RecoveryCodesGeneratedAt *time.Time `json:"recovery_codes_generated_at,omitempty"`
	AvailableMethods         []string   `json:"available_methods"`
}

type TOTPSetupResponse struct {
	SetupToken  string    `json:"setup_token"`
	Secret      string    `json:"secret"`
	Issuer      string    `json:"issuer"`
	AccountName string    `json:"account_name"`
	OtpAuthURI  string    `json:"otpauth_uri"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type SecurityVerificationInput struct {
	CurrentPassword string
	Code            string
	Method          string
	Purpose         string
}

type SecurityVerificationResponse struct {
	SecurityToken string    `json:"security_token"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type EnableTOTPInput struct {
	SetupToken string `json:"setup_token"`
	Code       string `json:"code"`
}

type VerifyMFARequest struct {
	MFAToken string `json:"mfa_token"`
	Code     string `json:"code"`
	Method   string `json:"method"`
}

type RecoveryCodesResponse struct {
	RecoveryCodes []string      `json:"recovery_codes"`
	GeneratedAt   time.Time     `json:"generated_at"`
	Status        MFAStatus     `json:"status"`
	Session       *AuthResponse `json:"session,omitempty"`
}

type MFAStatusResponse struct {
	Status  MFAStatus     `json:"status"`
	Session *AuthResponse `json:"session,omitempty"`
}

type mfaClaims struct {
	UserID      int      `json:"user_id"`
	Username    string   `json:"username,omitempty"`
	AuthVersion int64    `json:"auth_version"`
	Purpose     string   `json:"purpose"`
	Methods     []string `json:"methods,omitempty"`
	jwt.RegisteredClaims
}

func (s *AuthService) GetMFAStatus(ctx context.Context, userID int) (MFAStatus, error) {
	if _, err := s.getActiveUserByID(ctx, userID); err != nil {
		return MFAStatus{}, err
	}

	return s.getMFAStatusByUserID(ctx, int32(userID))
}

func (s *AuthService) BeginTOTPSetup(ctx context.Context, userID int, securityToken string) (TOTPSetupResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return TOTPSetupResponse{}, err
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		return TOTPSetupResponse{}, err
	}

	accountName := strings.TrimSpace(user.Username)

	encryptedSecret, err := s.encryptMFASecret(secret)
	if err != nil {
		return TOTPSetupResponse{}, fmt.Errorf("encrypt totp secret: %w", err)
	}

	enrollmentID := uuid.New().String()
	expiresAt := time.Now().UTC().Add(totpSetupTokenTTL)
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := s.consumeSecurityVerification(ctx, q, user, securityToken, securityPurposeTOTPSetup); err != nil {
			return err
		}
		if err := q.DeletePendingTOTPEnrollments(ctx, user.UserID); err != nil {
			return fmt.Errorf("replace pending totp enrollment: %w", err)
		}
		if _, err := q.CreatePendingTOTPEnrollment(ctx, repo.CreatePendingTOTPEnrollmentParams{
			EnrollmentID:     enrollmentID,
			UserID:           user.UserID,
			SecretCiphertext: encryptedSecret,
			AuthVersion:      user.AuthVersion,
			ExpiresAt:        dbtypes.NewTimestamp(expiresAt),
		}); err != nil {
			return fmt.Errorf("create pending totp enrollment: %w", err)
		}
		return nil
	}); err != nil {
		return TOTPSetupResponse{}, err
	}

	return TOTPSetupResponse{
		SetupToken:  enrollmentID,
		Secret:      secret,
		Issuer:      defaultTOTPIssuer,
		AccountName: accountName,
		OtpAuthURI:  buildTOTPAuthURI(defaultTOTPIssuer, accountName, secret),
		ExpiresAt:   expiresAt,
	}, nil
}

func (s *AuthService) EnableTOTP(ctx context.Context, userID int, input EnableTOTPInput) (RecoveryCodesResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	enrollmentID, err := uuid.Parse(strings.TrimSpace(input.SetupToken))
	if err != nil {
		return RecoveryCodesResponse{}, ErrInvalidMFAToken
	}

	pending, err := s.queries.GetPendingTOTPEnrollment(ctx, repo.GetPendingTOTPEnrollmentParams{
		EnrollmentID: enrollmentID.String(),
		UserID:       user.UserID,
		AuthVersion:  user.AuthVersion,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RecoveryCodesResponse{}, ErrExpiredMFAToken
		}
		return RecoveryCodesResponse{}, fmt.Errorf("get pending totp enrollment: %w", err)
	}

	secret, err := s.decryptMFASecret(pending.SecretCiphertext)
	if err != nil {
		return RecoveryCodesResponse{}, fmt.Errorf("decrypt pending totp secret: %w", err)
	}
	counter, valid := totpCodeCounter(secret, input.Code, time.Now())
	if !valid {
		return RecoveryCodesResponse{}, ErrInvalidMFACode
	}

	encryptedSecret := pending.SecretCiphertext

	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	var updatedUser repo.User
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		consumed, err := q.ConsumePendingTOTPEnrollment(ctx, repo.ConsumePendingTOTPEnrollmentParams{
			EnrollmentID: enrollmentID.String(),
			UserID:       user.UserID,
			AuthVersion:  user.AuthVersion,
		})
		if err != nil {
			return fmt.Errorf("consume pending totp enrollment: %w", err)
		}
		if consumed != 1 {
			return ErrInvalidMFAToken
		}
		if _, err := q.UpsertUserTOTPCredential(ctx, repo.UpsertUserTOTPCredentialParams{
			UserID:           user.UserID,
			SecretCiphertext: encryptedSecret,
			LastUsedCounter:  counter,
		}); err != nil {
			return fmt.Errorf("upsert totp credential: %w", err)
		}

		if err := q.DeleteUserRecoveryCodes(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}

		for _, hash := range recoveryHashes {
			if err := q.CreateUserRecoveryCode(ctx, repo.CreateUserRecoveryCodeParams{
				UserID:   user.UserID,
				CodeHash: hash,
			}); err != nil {
				return fmt.Errorf("create recovery code: %w", err)
			}
		}

		updatedUser, err = q.IncrementUserAuthVersion(ctx, user.UserID)
		if err != nil {
			return fmt.Errorf("advance authentication version: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, user.UserID); err != nil {
			return fmt.Errorf("revoke prior sessions: %w", err)
		}

		return nil
	}); err != nil {
		return RecoveryCodesResponse{}, err
	}

	status, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	generatedAt := time.Now()
	if status.RecoveryCodesGeneratedAt != nil {
		generatedAt = *status.RecoveryCodesGeneratedAt
	}

	session, err := s.generateAuthResponseWithAssurance(updatedUser, "mfa")
	if err != nil {
		return RecoveryCodesResponse{}, fmt.Errorf("issue replacement session: %w", err)
	}

	return RecoveryCodesResponse{
		RecoveryCodes: recoveryCodes,
		GeneratedAt:   generatedAt,
		Status:        status,
		Session:       session,
	}, nil
}

func (s *AuthService) DisableTOTP(ctx context.Context, userID int, securityToken string) (MFAStatusResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return MFAStatusResponse{}, err
	}

	currentStatus, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return MFAStatusResponse{}, err
	}
	if !currentStatus.TOTPEnabled {
		return MFAStatusResponse{}, ErrMFANotEnabled
	}

	var updatedUser repo.User
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := s.consumeSecurityVerification(ctx, q, user, securityToken, securityPurposeTOTPDisable); err != nil {
			return err
		}
		if err := q.DeleteUserRecoveryCodes(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}
		if err := q.DeleteUserTOTPCredential(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete totp credential: %w", err)
		}
		// Passkeys may only exist alongside TOTP — disabling TOTP disables MFA
		// entirely, so remove any passkeys to avoid a passkey-without-TOTP state.
		if err := q.DeleteUserWebAuthnCredentials(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete passkeys: %w", err)
		}
		var err error
		updatedUser, err = q.IncrementUserAuthVersion(ctx, user.UserID)
		if err != nil {
			return fmt.Errorf("advance authentication version: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, user.UserID); err != nil {
			return fmt.Errorf("revoke prior sessions: %w", err)
		}
		return nil
	}); err != nil {
		return MFAStatusResponse{}, err
	}

	status, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return MFAStatusResponse{}, err
	}
	session, err := s.generateAuthResponseWithAssurance(updatedUser, "password")
	if err != nil {
		return MFAStatusResponse{}, fmt.Errorf("issue replacement session: %w", err)
	}
	return MFAStatusResponse{Status: status, Session: session}, nil
}

func (s *AuthService) RegenerateRecoveryCodes(ctx context.Context, userID int, securityToken string) (RecoveryCodesResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	currentStatus, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}
	if !currentStatus.TOTPEnabled {
		return RecoveryCodesResponse{}, ErrMFANotEnabled
	}

	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	var updatedUser repo.User
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := s.consumeSecurityVerification(ctx, q, user, securityToken, securityPurposeRecoveryRegenerate); err != nil {
			return err
		}
		if err := q.DeleteUserRecoveryCodes(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}

		for _, hash := range recoveryHashes {
			if err := q.CreateUserRecoveryCode(ctx, repo.CreateUserRecoveryCodeParams{
				UserID:   user.UserID,
				CodeHash: hash,
			}); err != nil {
				return fmt.Errorf("create recovery code: %w", err)
			}
		}
		var err error
		updatedUser, err = q.IncrementUserAuthVersion(ctx, user.UserID)
		if err != nil {
			return fmt.Errorf("advance authentication version: %w", err)
		}
		if err := q.RevokeUserRefreshTokens(ctx, user.UserID); err != nil {
			return fmt.Errorf("revoke prior sessions: %w", err)
		}

		return nil
	}); err != nil {
		return RecoveryCodesResponse{}, err
	}

	status, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	generatedAt := time.Now()
	if status.RecoveryCodesGeneratedAt != nil {
		generatedAt = *status.RecoveryCodesGeneratedAt
	}

	session, err := s.generateAuthResponseWithAssurance(updatedUser, "mfa")
	if err != nil {
		return RecoveryCodesResponse{}, fmt.Errorf("issue replacement session: %w", err)
	}

	return RecoveryCodesResponse{
		RecoveryCodes: recoveryCodes,
		GeneratedAt:   generatedAt,
		Status:        status,
		Session:       session,
	}, nil
}

const (
	securityPurposeTOTPSetup          = "totp_setup"
	securityPurposeTOTPDisable        = "totp_disable"
	securityPurposeRecoveryRegenerate = "recovery_regenerate"
	securityPurposePasskeyMutation    = "passkey_mutation"
)

func (s *AuthService) VerifySecurity(ctx context.Context, userID int, input SecurityVerificationInput) (SecurityVerificationResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return SecurityVerificationResponse{}, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(input.CurrentPassword)); err != nil {
		return SecurityVerificationResponse{}, ErrInvalidCurrentSecret
	}
	if !validSecurityPurpose(input.Purpose) {
		return SecurityVerificationResponse{}, ErrInvalidSecurityProof
	}

	status, err := s.getMFAStatusByUserID(ctx, user.UserID)
	if err != nil {
		return SecurityVerificationResponse{}, err
	}

	rawToken, err := randomOpaqueBytes(32)
	if err != nil {
		return SecurityVerificationResponse{}, fmt.Errorf("generate security verification token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(rawToken)
	digest := sha256.Sum256([]byte(token))
	verificationID := uuid.New().String()
	expiresAt := time.Now().UTC().Add(securityVerificationTTL)
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		// Consume the existing factor and create its purpose-bound proof in one
		// transaction. A storage failure must not burn a valid TOTP/recovery
		// factor while leaving the caller without a proof to use.
		if status.TOTPEnabled {
			switch normalizeMFAMethod(input.Method) {
			case MFAMethodTOTP:
				if err := s.verifyUserTOTPWithQueries(ctx, q, user.UserID, input.Code); err != nil {
					if errors.Is(err, ErrInvalidMFACode) || errors.Is(err, ErrMFANotEnabled) {
						return ErrInvalidSecurityProof
					}
					return err
				}
			case MFAMethodRecoveryCode:
				if err := consumeRecoveryCodeWithQueries(ctx, q, user.UserID, input.Code); err != nil {
					if errors.Is(err, ErrInvalidMFACode) {
						return ErrInvalidSecurityProof
					}
					return err
				}
			default:
				return ErrInvalidSecurityProof
			}
		}

		if _, err := q.CreateAuthSecurityVerification(ctx, repo.CreateAuthSecurityVerificationParams{
			VerificationID: verificationID,
			TokenHash:      hex.EncodeToString(digest[:]),
			UserID:         user.UserID,
			AuthVersion:    user.AuthVersion,
			Purpose:        input.Purpose,
			ExpiresAt:      dbtypes.NewTimestamp(expiresAt),
		}); err != nil {
			return fmt.Errorf("store security verification: %w", err)
		}
		return nil
	}); err != nil {
		if errors.Is(err, ErrInvalidSecurityProof) {
			return SecurityVerificationResponse{}, err
		}
		return SecurityVerificationResponse{}, fmt.Errorf("store security verification: %w", err)
	}

	return SecurityVerificationResponse{SecurityToken: token, ExpiresAt: expiresAt}, nil
}

func validSecurityPurpose(purpose string) bool {
	switch purpose {
	case securityPurposeTOTPSetup, securityPurposeTOTPDisable, securityPurposeRecoveryRegenerate, securityPurposePasskeyMutation:
		return true
	default:
		return false
	}
}

func (s *AuthService) consumeSecurityVerification(ctx context.Context, q *repo.Queries, user repo.User, token, purpose string) error {
	if !validSecurityPurpose(purpose) || strings.TrimSpace(token) == "" {
		return ErrInvalidSecurityProof
	}
	digest := sha256.Sum256([]byte(strings.TrimSpace(token)))
	verification, err := q.GetAuthSecurityVerification(ctx, repo.GetAuthSecurityVerificationParams{
		TokenHash:   hex.EncodeToString(digest[:]),
		UserID:      user.UserID,
		AuthVersion: user.AuthVersion,
		Purpose:     purpose,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidSecurityProof
		}
		return fmt.Errorf("load security verification: %w", err)
	}
	consumed, err := q.ConsumeAuthSecurityVerification(ctx, repo.ConsumeAuthSecurityVerificationParams{
		VerificationID: verification.VerificationID,
		UserID:         user.UserID,
		AuthVersion:    user.AuthVersion,
		Purpose:        purpose,
	})
	if err != nil {
		return fmt.Errorf("consume security verification: %w", err)
	}
	if consumed != 1 {
		return ErrInvalidSecurityProof
	}
	return nil
}

func (s *AuthService) VerifyLoginMFA(ctx context.Context, req VerifyMFARequest) (*AuthResponse, error) {
	claims, err := s.parseMFAClaims(req.MFAToken, mfaTokenPurposeLogin)
	if err != nil {
		return nil, err
	}

	user, err := s.getActiveUserByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}
	if user.AuthVersion != claims.AuthVersion {
		return nil, ErrInvalidMFAToken
	}

	method := normalizeMFAMethod(req.Method)
	if !containsMFAMethod(claims.Methods, method) {
		return nil, ErrInvalidMFACode
	}
	switch method {
	case MFAMethodTOTP:
		if err := s.verifyUserTOTP(ctx, user.UserID, req.Code); err != nil {
			return nil, err
		}
	case MFAMethodRecoveryCode:
		if err := s.consumeRecoveryCode(ctx, user.UserID, req.Code); err != nil {
			return nil, err
		}
	default:
		return nil, ErrInvalidMFACode
	}

	// Factor verification and security mutations share the same auth-version
	// fence. Re-read after consuming the factor so a concurrent TOTP disable,
	// recovery rotation, password change, or passkey mutation cannot cause this
	// challenge to mint a session for the pre-mutation version.
	currentUser, err := s.getActiveUserByID(ctx, claims.UserID)
	if err != nil || currentUser.AuthVersion != claims.AuthVersion {
		return nil, ErrInvalidMFAToken
	}
	currentStatus, err := s.getMFAStatusByUserID(ctx, currentUser.UserID)
	if err != nil {
		return nil, ErrInvalidMFAToken
	}
	if !currentStatus.TOTPEnabled {
		return nil, ErrInvalidMFAToken
	}

	lastLogin, err := s.updateUserLastLogin(ctx, currentUser.UserID)
	if err != nil {
		fmt.Printf("Warning: failed to update last login for user %d: %v\n", currentUser.UserID, err)
	} else {
		currentUser.LastLogin = lastLogin
	}

	return s.generateAuthResponseWithAssurance(currentUser, "mfa")
}

func (s *AuthService) issueLoginMFAChallenge(user repo.User, status MFAStatus) (*AuthResponse, error) {
	methods := append([]string(nil), status.AvailableMethods...)
	token, err := s.issueMFAClaims(mfaClaims{
		UserID:      int(user.UserID),
		Username:    user.Username,
		AuthVersion: user.AuthVersion,
		Purpose:     mfaTokenPurposeLogin,
		Methods:     methods,
	}, loginMFAChallengeTTL)
	if err != nil {
		return nil, err
	}

	return &AuthResponse{
		User:        ptr(ConvertUserToResponse(user)),
		RequiresMFA: true,
		MFAToken:    token,
		MFAMethods:  methods,
	}, nil
}

func (s *AuthService) getMFAStatusByUserID(ctx context.Context, userID int32) (MFAStatus, error) {
	row, err := s.queries.GetUserMFAStatus(ctx, userID)
	if err != nil {
		return MFAStatus{}, fmt.Errorf("get mfa status: %w", err)
	}
	if row.TotpEnabled != 0 && row.TotpEnabled != 1 {
		return MFAStatus{}, ErrInvalidMFAState
	}
	if row.TotpEnabled == 0 && (row.PasskeyCount > 0 || row.RecoveryCodesRemaining > 0) {
		return MFAStatus{}, ErrInvalidMFAState
	}

	return buildMFAStatus(row), nil
}

func buildMFAStatus(row repo.GetUserMFAStatusRow) MFAStatus {
	var generatedAt *time.Time
	if row.RecoveryCodesGeneratedAt > 0 {
		value := time.UnixMicro(row.RecoveryCodesGeneratedAt).UTC()
		generatedAt = &value
	}
	return MFAStatus{
		TOTPEnabled:              row.TotpEnabled == 1,
		PasskeyCount:             int(row.PasskeyCount),
		RecoveryCodesRemaining:   int(row.RecoveryCodesRemaining),
		RecoveryCodesGeneratedAt: generatedAt,
		AvailableMethods:         availableMFAMethodsFromRow(row),
	}
}

func availableMFAMethodsFromRow(row repo.GetUserMFAStatusRow) []string {
	methods := make([]string, 0, 3)
	if row.TotpEnabled == 1 {
		if row.PasskeyCount > 0 {
			methods = append(methods, "passkey")
		}
		methods = append(methods, MFAMethodTOTP)
		if row.RecoveryCodesRemaining > 0 {
			methods = append(methods, MFAMethodRecoveryCode)
		}
	}
	return methods
}

func containsMFAMethod(methods []string, want string) bool {
	for _, method := range methods {
		if method == want {
			return true
		}
	}
	return false
}

func optionalDBTimestamp(value dbtypes.Timestamp) *time.Time {
	if !value.Valid {
		return nil
	}
	copied := value.Time
	return &copied
}

func (s *AuthService) getActiveUserByID(ctx context.Context, userID int) (repo.User, error) {
	user, err := s.queries.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repo.User{}, ErrUserNotFound
		}
		return repo.User{}, fmt.Errorf("get user by id: %w", err)
	}

	if !user.IsActive {
		return repo.User{}, ErrUserNotFound
	}

	return user, nil
}

func (s *AuthService) verifyUserTOTP(ctx context.Context, userID int32, code string) error {
	return s.withTx(ctx, func(q *repo.Queries) error {
		return s.verifyUserTOTPWithQueries(ctx, q, userID, code)
	})
}

func (s *AuthService) verifyUserTOTPWithQueries(ctx context.Context, q *repo.Queries, userID int32, code string) error {
	credential, err := q.GetUserTOTPCredential(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrMFANotEnabled
		}
		return fmt.Errorf("get totp credential: %w", err)
	}

	secret, err := s.decryptMFASecret(credential.SecretCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt totp secret: %w", err)
	}
	counter, ok := totpCodeCounter(secret, code, time.Now())
	if !ok {
		return ErrInvalidMFACode
	}

	updated, err := q.UseTOTPCode(ctx, repo.UseTOTPCodeParams{UserID: userID, LastUsedCounter: counter})
	if err != nil {
		return fmt.Errorf("record TOTP counter: %w", err)
	}
	if updated != 1 {
		return ErrInvalidMFACode
	}
	return nil
}

func (s *AuthService) consumeRecoveryCode(ctx context.Context, userID int32, code string) error {
	return s.withTx(ctx, func(q *repo.Queries) error {
		return consumeRecoveryCodeWithQueries(ctx, q, userID, code)
	})
}

func consumeRecoveryCodeWithQueries(ctx context.Context, q *repo.Queries, userID int32, code string) error {
	hash := hashRecoveryCode(code)
	if hash == "" {
		return ErrInvalidMFACode
	}

	if _, err := q.UseRecoveryCode(ctx, repo.UseRecoveryCodeParams{
		UserID:   userID,
		CodeHash: hash,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidMFACode
		}
		return fmt.Errorf("use recovery code: %w", err)
	}

	return nil
}

func (s *AuthService) issueMFAClaims(claims mfaClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "lumilio-photos",
		Subject:   strconv.Itoa(claims.UserID),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.mfaTokenSecret)
}

func (s *AuthService) parseMFAClaims(tokenString string, expectedPurpose string) (*mfaClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &mfaClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.mfaTokenSecret, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired), errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, ErrExpiredMFAToken
		default:
			return nil, ErrInvalidMFAToken
		}
	}

	claims, ok := token.Claims.(*mfaClaims)
	if !ok || !token.Valid || claims.Purpose != expectedPurpose {
		return nil, ErrInvalidMFAToken
	}

	return claims, nil
}

func normalizeMFAMethod(method string) string {
	switch strings.ToLower(strings.TrimSpace(method)) {
	case MFAMethodTOTP:
		return MFAMethodTOTP
	case MFAMethodRecoveryCode:
		return MFAMethodRecoveryCode
	default:
		return ""
	}
}

func buildTOTPAuthURI(issuer string, accountName string, secret string) string {
	values := url.Values{}
	values.Set("secret", secret)
	values.Set("issuer", issuer)
	values.Set("algorithm", "SHA1")
	values.Set("digits", strconv.Itoa(totpDigits))
	values.Set("period", strconv.Itoa(int(totpPeriod/time.Second)))

	return (&url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + issuer + ":" + accountName,
		RawQuery: values.Encode(),
	}).String()
}

func (s *AuthService) encryptMFASecret(plaintext string) ([]byte, error) {
	block, err := aes.NewCipher(s.mfaEncryptKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}

	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	return aead.Seal(nonce, nonce, []byte(plaintext), nil), nil
}

func (s *AuthService) decryptMFASecret(ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(s.mfaEncryptKey)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create gcm: %w", err)
	}

	if len(ciphertext) < aead.NonceSize() {
		return "", errors.New("invalid ciphertext")
	}

	nonce := ciphertext[:aead.NonceSize()]
	payload := ciphertext[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt mfa secret: %w", err)
	}

	return string(plaintext), nil
}

func (s *AuthService) withTx(ctx context.Context, fn func(*repo.Queries) error) error {
	if s.db == nil {
		return fn(s.queries)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
