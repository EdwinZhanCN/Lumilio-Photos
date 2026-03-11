package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultTOTPIssuer      = "Lumilio Photos"
	loginMFAChallengeTTL   = 5 * time.Minute
	totpSetupTokenTTL      = 10 * time.Minute
	mfaTokenPurposeLogin   = "mfa_login"
	mfaTokenPurposeTOTPSet = "totp_setup"
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
)

type MFAStatus struct {
	TOTPEnabled              bool       `json:"totp_enabled"`
	PasskeyCount             int        `json:"passkey_count"`
	RecoveryCodesRemaining   int        `json:"recovery_codes_remaining"`
	RecoveryCodesGeneratedAt *time.Time `json:"recovery_codes_generated_at,omitempty"`
	AvailableMethods         []string   `json:"available_methods"`
}

type TOTPSetupResponse struct {
	SetupToken  string `json:"setup_token"`
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
	OtpAuthURI  string `json:"otpauth_uri"`
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
	RecoveryCodes []string  `json:"recovery_codes"`
	GeneratedAt   time.Time `json:"generated_at"`
	Status        MFAStatus `json:"status"`
}

type mfaClaims struct {
	UserID   int      `json:"user_id"`
	Username string   `json:"username,omitempty"`
	Purpose  string   `json:"purpose"`
	Secret   string   `json:"secret,omitempty"`
	Methods  []string `json:"methods,omitempty"`
	jwt.RegisteredClaims
}

func (s *AuthService) GetMFAStatus(ctx context.Context, userID int) (MFAStatus, error) {
	if _, err := s.getActiveUserByID(ctx, userID); err != nil {
		return MFAStatus{}, err
	}

	return s.getMFAStatusByUserID(ctx, int32(userID))
}

func (s *AuthService) BeginTOTPSetup(ctx context.Context, userID int) (TOTPSetupResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return TOTPSetupResponse{}, err
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		return TOTPSetupResponse{}, err
	}

	accountName := strings.TrimSpace(user.Username)

	setupToken, err := s.issueMFAClaims(mfaClaims{
		UserID:   int(user.UserID),
		Username: user.Username,
		Purpose:  mfaTokenPurposeTOTPSet,
		Secret:   secret,
	}, totpSetupTokenTTL)
	if err != nil {
		return TOTPSetupResponse{}, fmt.Errorf("issue totp setup token: %w", err)
	}

	return TOTPSetupResponse{
		SetupToken:  setupToken,
		Secret:      secret,
		Issuer:      defaultTOTPIssuer,
		AccountName: accountName,
		OtpAuthURI:  buildTOTPAuthURI(defaultTOTPIssuer, accountName, secret),
	}, nil
}

func (s *AuthService) EnableTOTP(ctx context.Context, userID int, input EnableTOTPInput) (RecoveryCodesResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	claims, err := s.parseMFAClaims(input.SetupToken, mfaTokenPurposeTOTPSet)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}
	if claims.UserID != int(user.UserID) || strings.TrimSpace(claims.Secret) == "" {
		return RecoveryCodesResponse{}, ErrInvalidMFAToken
	}

	if !validateTOTPCode(claims.Secret, input.Code, time.Now()) {
		return RecoveryCodesResponse{}, ErrInvalidMFACode
	}

	encryptedSecret, err := s.encryptMFASecret(claims.Secret)
	if err != nil {
		return RecoveryCodesResponse{}, fmt.Errorf("encrypt totp secret: %w", err)
	}

	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if _, err := q.UpsertUserTOTPCredential(ctx, repo.UpsertUserTOTPCredentialParams{
			UserID:           user.UserID,
			SecretCiphertext: encryptedSecret,
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

	return RecoveryCodesResponse{
		RecoveryCodes: recoveryCodes,
		GeneratedAt:   generatedAt,
		Status:        status,
	}, nil
}

func (s *AuthService) DisableTOTP(ctx context.Context, userID int, currentPassword string) (MFAStatus, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return MFAStatus{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return MFAStatus{}, ErrInvalidCurrentSecret
	}

	if _, err := s.queries.GetUserTOTPCredential(ctx, user.UserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return MFAStatus{}, ErrMFANotEnabled
		}
		return MFAStatus{}, fmt.Errorf("get totp credential: %w", err)
	}

	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if err := q.DeleteUserRecoveryCodes(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete recovery codes: %w", err)
		}
		if err := q.DeleteUserTOTPCredential(ctx, user.UserID); err != nil {
			return fmt.Errorf("delete totp credential: %w", err)
		}
		return nil
	}); err != nil {
		return MFAStatus{}, err
	}

	return s.getMFAStatusByUserID(ctx, user.UserID)
}

func (s *AuthService) RegenerateRecoveryCodes(ctx context.Context, userID int, currentPassword string) (RecoveryCodesResponse, error) {
	user, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPassword)); err != nil {
		return RecoveryCodesResponse{}, ErrInvalidCurrentSecret
	}

	if _, err := s.queries.GetUserTOTPCredential(ctx, user.UserID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecoveryCodesResponse{}, ErrMFANotEnabled
		}
		return RecoveryCodesResponse{}, fmt.Errorf("get totp credential: %w", err)
	}

	recoveryCodes, recoveryHashes, err := generateRecoveryCodes()
	if err != nil {
		return RecoveryCodesResponse{}, err
	}

	if err := s.withTx(ctx, func(q *repo.Queries) error {
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

	return RecoveryCodesResponse{
		RecoveryCodes: recoveryCodes,
		GeneratedAt:   generatedAt,
		Status:        status,
	}, nil
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

	method := normalizeMFAMethod(req.Method)
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

	lastLogin, err := s.updateUserLastLogin(ctx, user.UserID)
	if err != nil {
		fmt.Printf("Warning: failed to update last login for user %d: %v\n", user.UserID, err)
	} else {
		user.LastLogin = lastLogin
	}

	return s.generateAuthResponse(user)
}

func (s *AuthService) issueLoginMFAChallenge(user repo.User, status repo.GetUserMFAStatusRow) (*AuthResponse, error) {
	methods := availableMFAMethodsFromRow(status)
	token, err := s.issueMFAClaims(mfaClaims{
		UserID:   int(user.UserID),
		Username: user.Username,
		Purpose:  mfaTokenPurposeLogin,
		Methods:  append([]string(nil), methods...),
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

	return buildMFAStatus(row), nil
}

func buildMFAStatus(row repo.GetUserMFAStatusRow) MFAStatus {
	return MFAStatus{
		TOTPEnabled:              coerceBool(row.TotpEnabled),
		PasskeyCount:             int(row.PasskeyCount),
		RecoveryCodesRemaining:   int(row.RecoveryCodesRemaining),
		RecoveryCodesGeneratedAt: coerceOptionalTime(row.RecoveryCodesGeneratedAt),
		AvailableMethods:         availableMFAMethodsFromRow(row),
	}
}

func availableMFAMethodsFromRow(row repo.GetUserMFAStatusRow) []string {
	methods := make([]string, 0, 3)
	if row.PasskeyCount > 0 {
		methods = append(methods, "passkey")
	}
	if coerceBool(row.TotpEnabled) {
		methods = append(methods, MFAMethodTOTP)
		if row.RecoveryCodesRemaining > 0 {
			methods = append(methods, MFAMethodRecoveryCode)
		}
	}
	return methods
}

func coerceBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case *bool:
		return typed != nil && *typed
	default:
		return false
	}
}

func coerceOptionalTime(value interface{}) *time.Time {
	switch typed := value.(type) {
	case nil:
		return nil
	case time.Time:
		copied := typed
		return &copied
	case *time.Time:
		if typed == nil {
			return nil
		}
		copied := *typed
		return &copied
	case pgtype.Timestamptz:
		if !typed.Valid {
			return nil
		}
		copied := typed.Time
		return &copied
	case *pgtype.Timestamptz:
		if typed == nil || !typed.Valid {
			return nil
		}
		copied := typed.Time
		return &copied
	default:
		return nil
	}
}

func (s *AuthService) getActiveUserByID(ctx context.Context, userID int) (repo.User, error) {
	user, err := s.queries.GetUserByID(ctx, int32(userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.User{}, ErrUserNotFound
		}
		return repo.User{}, fmt.Errorf("get user by id: %w", err)
	}

	if user.IsActive == nil || !*user.IsActive {
		return repo.User{}, ErrUserNotFound
	}

	return user, nil
}

func (s *AuthService) verifyUserTOTP(ctx context.Context, userID int32, code string) error {
	credential, err := s.queries.GetUserTOTPCredential(ctx, userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrMFANotEnabled
		}
		return fmt.Errorf("get totp credential: %w", err)
	}

	secret, err := s.decryptMFASecret(credential.SecretCiphertext)
	if err != nil {
		return fmt.Errorf("decrypt totp secret: %w", err)
	}

	if !validateTOTPCode(secret, code, time.Now()) {
		return ErrInvalidMFACode
	}

	if err := s.queries.UpdateUserTOTPLastUsed(ctx, userID); err != nil {
		fmt.Printf("Warning: failed to update TOTP last-used timestamp for user %d: %v\n", userID, err)
	}

	return nil
}

func (s *AuthService) consumeRecoveryCode(ctx context.Context, userID int32, code string) error {
	hash := hashRecoveryCode(code)
	if hash == "" {
		return ErrInvalidMFACode
	}

	if _, err := s.queries.UseRecoveryCode(ctx, repo.UseRecoveryCodeParams{
		UserID:   userID,
		CodeHash: hash,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
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
	case MFAMethodRecoveryCode:
		return MFAMethodRecoveryCode
	default:
		return MFAMethodTOTP
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

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(s.queries.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
