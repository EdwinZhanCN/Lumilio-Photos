package service

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"server/internal/db/repo"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	passkeyChallengeTTL           = 10 * time.Minute
	defaultWebAuthnRPDisplayName  = "Lumilio"
	passkeyTokenPurposeLogin      = "passkey_login"
	passkeyTokenPurposeEnroll     = "passkey_enroll"
	defaultPasskeyBytes           = 32
	defaultPasskeyCredentialLabel = "Passkey"
)

var (
	ErrInvalidPasskeyChallenge   = errors.New("invalid passkey challenge")
	ErrExpiredPasskeyChallenge   = errors.New("expired passkey challenge")
	ErrPasskeyNotConfigured      = errors.New("passkey is not configured")
	ErrPasskeyCredentialNotFound = errors.New("passkey credential not found")
	// ErrTOTPRequiredForPasskey enforces the invariant that a passkey can only
	// exist alongside TOTP — so every passkey account always has a usable
	// password + TOTP fallback and no factor-less bypass can occur.
	ErrTOTPRequiredForPasskey = errors.New("totp must be enabled before adding a passkey")
)

type RegistrationStartRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type PasskeyOptionsResponse struct {
	Options        any    `json:"options"`
	ChallengeToken string `json:"challenge_token"`
}

type PasskeyCredentialSummary struct {
	PasskeyID  int        `json:"passkey_id"`
	Label      string     `json:"label"`
	Transports []string   `json:"transports,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type PasskeyListResponse struct {
	Credentials []PasskeyCredentialSummary `json:"credentials"`
	Total       int                        `json:"total"`
}

type passkeyChallengeClaims struct {
	Purpose               string               `json:"purpose"`
	Origin                string               `json:"origin"`
	UserID                int                  `json:"user_id,omitempty"`
	Username              string               `json:"username,omitempty"`
	RegistrationSessionID string               `json:"registration_session_id,omitempty"`
	SessionData           webauthn.SessionData `json:"session_data"`
	jwt.RegisteredClaims
}

type webAuthnUser struct {
	id          []byte
	username    string
	displayName string
	credentials []webauthn.Credential
}

func (u webAuthnUser) WebAuthnID() []byte {
	return u.id
}

func (u webAuthnUser) WebAuthnName() string {
	return u.username
}

func (u webAuthnUser) WebAuthnDisplayName() string {
	if strings.TrimSpace(u.displayName) == "" {
		return u.username
	}
	return u.displayName
}

func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	return append([]webauthn.Credential(nil), u.credentials...)
}

func loadWebAuthnAllowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("SERVER_CORS_ALLOWED_ORIGINS"))
	}
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalized, _, err := normalizeOriginString(trimmed)
		if err != nil {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		origins = append(origins, normalized)
	}

	return origins
}

func loadWebAuthnRPDisplayName() string {
	if value := strings.TrimSpace(os.Getenv("WEBAUTHN_RP_NAME")); value != "" {
		return value
	}
	return defaultWebAuthnRPDisplayName
}

func loadWebAuthnRPID() string {
	return strings.TrimSpace(os.Getenv("WEBAUTHN_RP_ID"))
}

// StartRegistration creates a new account from a username and password and
// immediately logs it in. MFA (TOTP and/or passkey) is fully optional and is
// added afterwards via the authenticated enrollment endpoints.
func (s *AuthService) StartRegistration(ctx context.Context, req RegistrationStartRequest) (*AuthResponse, error) {
	username, err := normalizeUsername(req.Username)
	if err != nil {
		return nil, err
	}

	passwordHash, err := bcryptPassword(req.Password)
	if err != nil {
		return nil, err
	}

	userHandle, err := randomOpaqueBytes(defaultPasskeyBytes)
	if err != nil {
		return nil, fmt.Errorf("generate webauthn user handle: %w", err)
	}

	var (
		createdUser repo.User
		finalRole   UserRole
	)
	if err := s.withTx(ctx, func(q *repo.Queries) error {
		if _, err := q.GetUserByUsername(ctx, username); err == nil {
			return ErrUserAlreadyExists
		} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("check username availability: %w", err)
		}

		// First account becomes the admin; everyone after is a regular user.
		role, err := determineRegistrationRole(ctx, q, string(UserRoleAdmin))
		if err != nil {
			return err
		}
		finalRole = role

		user, err := q.CreateUser(ctx, repo.CreateUserParams{
			Username:           username,
			Password:           passwordHash,
			DisplayName:        username,
			Role:               string(role),
			WebauthnUserHandle: userHandle,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		createdUser = user

		// Assign primary repository owner on first user creation.
		ownerID := user.UserID
		if _, ownerErr := q.SetPrimaryRepositoryOwner(ctx, &ownerID); ownerErr != nil {
			if !errors.Is(ownerErr, pgx.ErrNoRows) {
				return fmt.Errorf("set primary repository owner: %w", ownerErr)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	response, err := s.generateAuthResponse(createdUser)
	if err != nil {
		return nil, err
	}
	response.BootstrapAdmin = finalRole == UserRoleAdmin
	return response, nil
}

func (s *AuthService) BeginPasskeyLogin(ctx context.Context, username string, origin string) (PasskeyOptionsResponse, error) {
	userRecord, user, err := s.getUserForPasskey(ctx, username)
	if err != nil {
		return PasskeyOptionsResponse{}, err
	}

	wa, normalizedOrigin, err := s.newWebAuthnForOrigin(origin)
	if err != nil {
		return PasskeyOptionsResponse{}, err
	}

	assertion, sessionData, err := wa.BeginLogin(
		user,
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return PasskeyOptionsResponse{}, ErrPasskeyNotConfigured
	}

	challengeToken, err := s.issuePasskeyChallenge(passkeyChallengeClaims{
		Purpose:     passkeyTokenPurposeLogin,
		Origin:      normalizedOrigin,
		UserID:      int(userRecord.UserID),
		Username:    userRecord.Username,
		SessionData: *sessionData,
	}, passkeyChallengeTTL)
	if err != nil {
		return PasskeyOptionsResponse{}, fmt.Errorf("issue passkey challenge: %w", err)
	}

	return PasskeyOptionsResponse{
		Options:        assertion,
		ChallengeToken: challengeToken,
	}, nil
}

func (s *AuthService) VerifyPasskeyLogin(ctx context.Context, challengeToken string, credentialJSON []byte) (*AuthResponse, error) {
	challenge, err := s.parsePasskeyChallenge(challengeToken, passkeyTokenPurposeLogin)
	if err != nil {
		return nil, err
	}

	userRecord, err := s.getActiveUserByID(ctx, challenge.UserID)
	if err != nil {
		return nil, ErrPasskeyNotConfigured
	}

	passkeys, err := s.queries.ListUserWebAuthnCredentials(ctx, userRecord.UserID)
	if err != nil {
		return nil, fmt.Errorf("load passkeys: %w", err)
	}
	if len(passkeys) == 0 {
		return nil, ErrPasskeyNotConfigured
	}

	wa, err := s.newWebAuthnForChallenge(challenge)
	if err != nil {
		return nil, err
	}

	parsed, err := protocol.ParseCredentialRequestResponseBytes(credentialJSON)
	if err != nil {
		return nil, ErrInvalidPasskeyChallenge
	}

	validatedCredential, err := wa.ValidateLogin(userToWebAuthnUser(userRecord, passkeys), challenge.SessionData, parsed)
	if err != nil {
		return nil, ErrInvalidPasskeyChallenge
	}

	if _, err := s.queries.UpdateUserWebAuthnCredentialUsage(ctx, credentialToUsageParams(userRecord.UserID, *validatedCredential)); err != nil {
		return nil, fmt.Errorf("update passkey usage: %w", err)
	}

	lastLogin, err := s.updateUserLastLogin(ctx, userRecord.UserID)
	if err == nil {
		userRecord.LastLogin = lastLogin
	}

	return s.generateAuthResponse(userRecord)
}

func (s *AuthService) BeginPasskeyEnrollment(ctx context.Context, userID int, origin string) (PasskeyOptionsResponse, error) {
	userRecord, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return PasskeyOptionsResponse{}, err
	}

	// A passkey may only be added once TOTP is enabled, so every passkey account
	// keeps a password + TOTP fallback.
	if enabled, err := s.userHasTOTP(ctx, userRecord.UserID); err != nil {
		return PasskeyOptionsResponse{}, err
	} else if !enabled {
		return PasskeyOptionsResponse{}, ErrTOTPRequiredForPasskey
	}

	passkeys, err := s.queries.ListUserWebAuthnCredentials(ctx, userRecord.UserID)
	if err != nil {
		return PasskeyOptionsResponse{}, fmt.Errorf("load passkeys: %w", err)
	}

	wa, normalizedOrigin, err := s.newWebAuthnForOrigin(origin)
	if err != nil {
		return PasskeyOptionsResponse{}, err
	}

	user := userToWebAuthnUser(userRecord, passkeys)
	creation, sessionData, err := wa.BeginRegistration(
		user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithExclusions(webauthn.Credentials(user.WebAuthnCredentials()).CredentialDescriptors()),
		webauthn.WithExtensions(map[string]any{"credProps": true}),
	)
	if err != nil {
		return PasskeyOptionsResponse{}, fmt.Errorf("begin passkey enrollment: %w", err)
	}

	challengeToken, err := s.issuePasskeyChallenge(passkeyChallengeClaims{
		Purpose:     passkeyTokenPurposeEnroll,
		Origin:      normalizedOrigin,
		UserID:      userID,
		Username:    userRecord.Username,
		SessionData: *sessionData,
	}, passkeyChallengeTTL)
	if err != nil {
		return PasskeyOptionsResponse{}, fmt.Errorf("issue passkey enrollment challenge: %w", err)
	}

	return PasskeyOptionsResponse{
		Options:        creation,
		ChallengeToken: challengeToken,
	}, nil
}

func (s *AuthService) VerifyPasskeyEnrollment(ctx context.Context, userID int, challengeToken string, credentialJSON []byte) (PasskeyCredentialSummary, error) {
	challenge, err := s.parsePasskeyChallenge(challengeToken, passkeyTokenPurposeEnroll)
	if err != nil {
		return PasskeyCredentialSummary{}, err
	}
	if challenge.UserID != userID {
		return PasskeyCredentialSummary{}, ErrInvalidPasskeyChallenge
	}

	userRecord, err := s.getActiveUserByID(ctx, userID)
	if err != nil {
		return PasskeyCredentialSummary{}, err
	}

	if enabled, err := s.userHasTOTP(ctx, userRecord.UserID); err != nil {
		return PasskeyCredentialSummary{}, err
	} else if !enabled {
		return PasskeyCredentialSummary{}, ErrTOTPRequiredForPasskey
	}

	passkeys, err := s.queries.ListUserWebAuthnCredentials(ctx, userRecord.UserID)
	if err != nil {
		return PasskeyCredentialSummary{}, fmt.Errorf("load passkeys: %w", err)
	}

	wa, err := s.newWebAuthnForChallenge(challenge)
	if err != nil {
		return PasskeyCredentialSummary{}, err
	}

	parsed, err := protocol.ParseCredentialCreationResponseBytes(credentialJSON)
	if err != nil {
		return PasskeyCredentialSummary{}, ErrInvalidPasskeyChallenge
	}

	credential, err := wa.CreateCredential(userToWebAuthnUser(userRecord, passkeys), challenge.SessionData, parsed)
	if err != nil {
		return PasskeyCredentialSummary{}, ErrInvalidPasskeyChallenge
	}

	row, err := s.queries.CreateUserWebAuthnCredential(ctx, credentialToCreateParams(userRecord.UserID, *credential))
	if err != nil {
		return PasskeyCredentialSummary{}, fmt.Errorf("create passkey: %w", err)
	}

	return passkeySummaryFromRow(row, len(passkeys)+1)
}

func (s *AuthService) ListPasskeys(ctx context.Context, userID int) (PasskeyListResponse, error) {
	if _, err := s.getActiveUserByID(ctx, userID); err != nil {
		return PasskeyListResponse{}, err
	}

	rows, err := s.queries.ListUserWebAuthnCredentialSummaries(ctx, int32(userID))
	if err != nil {
		return PasskeyListResponse{}, fmt.Errorf("list passkeys: %w", err)
	}

	items := make([]PasskeyCredentialSummary, 0, len(rows))
	for i, row := range rows {
		item, err := passkeySummaryFromListRow(row, i+1)
		if err != nil {
			return PasskeyListResponse{}, err
		}
		items = append(items, item)
	}

	return PasskeyListResponse{
		Credentials: items,
		Total:       len(items),
	}, nil
}

func (s *AuthService) DeletePasskey(ctx context.Context, userID int, passkeyID int) error {
	if _, err := s.getActiveUserByID(ctx, userID); err != nil {
		return err
	}

	rowsAffected, err := s.queries.DeleteUserWebAuthnCredential(ctx, repo.DeleteUserWebAuthnCredentialParams{
		UserID:                   int32(userID),
		UserWebauthnCredentialID: int32(passkeyID),
	})
	if err != nil {
		return fmt.Errorf("delete passkey: %w", err)
	}
	if rowsAffected == 0 {
		return ErrPasskeyCredentialNotFound
	}

	return nil
}

// userHasTOTP reports whether the user has an active TOTP credential.
func (s *AuthService) userHasTOTP(ctx context.Context, userID int32) (bool, error) {
	if _, err := s.queries.GetUserTOTPCredential(ctx, userID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("get totp credential: %w", err)
	}
	return true, nil
}

func (s *AuthService) getUserForPasskey(ctx context.Context, username string) (repo.User, webAuthnUser, error) {
	user, err := s.queries.GetUserByUsername(ctx, strings.ToLower(strings.TrimSpace(username)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.User{}, webAuthnUser{}, ErrPasskeyNotConfigured
		}
		return repo.User{}, webAuthnUser{}, fmt.Errorf("get user by username: %w", err)
	}
	if user.IsActive == nil || !*user.IsActive {
		return repo.User{}, webAuthnUser{}, ErrPasskeyNotConfigured
	}

	rows, err := s.queries.ListUserWebAuthnCredentials(ctx, user.UserID)
	if err != nil {
		return repo.User{}, webAuthnUser{}, fmt.Errorf("list user passkeys: %w", err)
	}
	if len(rows) == 0 {
		return repo.User{}, webAuthnUser{}, ErrPasskeyNotConfigured
	}

	return user, userToWebAuthnUser(user, rows), nil
}

func determineRegistrationRole(ctx context.Context, q *repo.Queries, desiredRole string) (UserRole, error) {
	role := normalizeUserRole(desiredRole)
	if role != UserRoleAdmin {
		return UserRoleUser, nil
	}

	count, err := q.CountUsers(ctx)
	if err != nil {
		return UserRoleUser, fmt.Errorf("count users: %w", err)
	}
	if count == 0 {
		return UserRoleAdmin, nil
	}

	return UserRoleUser, nil
}

func userToWebAuthnUser(user repo.User, rows []repo.UserWebauthnCredential) webAuthnUser {
	credentials := make([]webauthn.Credential, 0, len(rows))
	for _, row := range rows {
		credentials = append(credentials, passkeyRowToCredential(row))
	}

	displayName := strings.TrimSpace(user.DisplayName)
	if displayName == "" {
		displayName = user.Username
	}

	return webAuthnUser{
		id:          cloneByteSlice(user.WebauthnUserHandle),
		username:    user.Username,
		displayName: displayName,
		credentials: credentials,
	}
}

func passkeyRowToCredential(row repo.UserWebauthnCredential) webauthn.Credential {
	return webauthn.Credential{
		ID:              cloneByteSlice(row.CredentialID),
		PublicKey:       cloneByteSlice(row.PublicKey),
		AttestationType: row.AttestationType,
		Transport:       decodeCredentialTransports(row.Transports),
		Flags: webauthn.CredentialFlags{
			UserPresent:    row.UserPresent,
			UserVerified:   row.UserVerified,
			BackupEligible: row.BackupEligible,
			BackupState:    row.BackupState,
		},
		Authenticator: webauthn.Authenticator{
			AAGUID:    cloneByteSlice(row.Aaguid),
			SignCount: uint32(row.SignCount),
		},
	}
}

func credentialToCreateParams(userID int32, credential webauthn.Credential) repo.CreateUserWebAuthnCredentialParams {
	return repo.CreateUserWebAuthnCredentialParams{
		CredentialID:    cloneByteSlice(credential.ID),
		UserID:          userID,
		PublicKey:       cloneByteSlice(credential.PublicKey),
		SignCount:       int64(credential.Authenticator.SignCount),
		Transports:      encodeCredentialTransports(credential.Transport),
		AttestationType: credential.AttestationType,
		Aaguid:          cloneByteSlice(credential.Authenticator.AAGUID),
		UserPresent:     credential.Flags.UserPresent,
		UserVerified:    credential.Flags.UserVerified,
		BackupEligible:  credential.Flags.BackupEligible,
		BackupState:     credential.Flags.BackupState,
	}
}

func credentialToUsageParams(userID int32, credential webauthn.Credential) repo.UpdateUserWebAuthnCredentialUsageParams {
	return repo.UpdateUserWebAuthnCredentialUsageParams{
		UserID:         userID,
		CredentialID:   cloneByteSlice(credential.ID),
		SignCount:      int64(credential.Authenticator.SignCount),
		Transports:     encodeCredentialTransports(credential.Transport),
		UserPresent:    credential.Flags.UserPresent,
		UserVerified:   credential.Flags.UserVerified,
		BackupEligible: credential.Flags.BackupEligible,
		BackupState:    credential.Flags.BackupState,
	}
}

func encodeCredentialTransports(transports []protocol.AuthenticatorTransport) []byte {
	if len(transports) == 0 {
		return []byte("[]")
	}

	values := make([]string, 0, len(transports))
	for _, transport := range transports {
		values = append(values, string(transport))
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return []byte("[]")
	}
	return payload
}

func decodeCredentialTransports(payload []byte) []protocol.AuthenticatorTransport {
	if len(payload) == 0 {
		return nil
	}

	var values []string
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil
	}

	transports := make([]protocol.AuthenticatorTransport, 0, len(values))
	for _, value := range values {
		transports = append(transports, protocol.AuthenticatorTransport(value))
	}
	return transports
}

func passkeySummaryFromRow(row repo.UserWebauthnCredential, ordinal int) (PasskeyCredentialSummary, error) {
	transports, err := decodeTransportLabels(row.Transports)
	if err != nil {
		return PasskeyCredentialSummary{}, err
	}

	return PasskeyCredentialSummary{
		PasskeyID:  int(row.UserWebauthnCredentialID),
		Label:      fmt.Sprintf("%s %d", defaultPasskeyCredentialLabel, ordinal),
		Transports: transports,
		CreatedAt:  row.CreatedAt.Time,
		LastUsedAt: coerceOptionalTime(row.LastUsedAt),
	}, nil
}

func passkeySummaryFromListRow(row repo.ListUserWebAuthnCredentialSummariesRow, ordinal int) (PasskeyCredentialSummary, error) {
	transports, err := decodeTransportLabels(row.Transports)
	if err != nil {
		return PasskeyCredentialSummary{}, err
	}

	return PasskeyCredentialSummary{
		PasskeyID:  int(row.UserWebauthnCredentialID),
		Label:      fmt.Sprintf("%s %d", defaultPasskeyCredentialLabel, ordinal),
		Transports: transports,
		CreatedAt:  row.CreatedAt.Time,
		LastUsedAt: coerceOptionalTime(row.LastUsedAt),
	}, nil
}

func decodeTransportLabels(payload []byte) ([]string, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	var values []string
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, fmt.Errorf("decode passkey transports: %w", err)
	}
	return values, nil
}

func (s *AuthService) issuePasskeyChallenge(claims passkeyChallengeClaims, ttl time.Duration) (string, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Issuer:    "lumilio-photos",
		Subject:   claims.Username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.passkeyTokenSecret)
}

func (s *AuthService) parsePasskeyChallenge(tokenString string, expectedPurpose string) (*passkeyChallengeClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &passkeyChallengeClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.passkeyTokenSecret, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired), errors.Is(err, jwt.ErrTokenNotValidYet):
			return nil, ErrExpiredPasskeyChallenge
		default:
			return nil, ErrInvalidPasskeyChallenge
		}
	}

	claims, ok := token.Claims.(*passkeyChallengeClaims)
	if !ok || !token.Valid || claims.Purpose != expectedPurpose {
		return nil, ErrInvalidPasskeyChallenge
	}

	return claims, nil
}

func (s *AuthService) newWebAuthnForOrigin(origin string) (*webauthn.WebAuthn, string, error) {
	normalizedOrigin, parsedOrigin, err := s.normalizeWebAuthnOrigin(origin)
	if err != nil {
		return nil, "", err
	}

	rpID, err := s.resolveWebAuthnRPID(parsedOrigin)
	if err != nil {
		return nil, "", err
	}

	instance, err := s.newWebAuthnInstance(normalizedOrigin, rpID)
	if err != nil {
		return nil, "", err
	}

	return instance, normalizedOrigin, nil
}

func (s *AuthService) newWebAuthnForChallenge(challenge *passkeyChallengeClaims) (*webauthn.WebAuthn, error) {
	normalizedOrigin, _, err := s.normalizeWebAuthnOrigin(challenge.Origin)
	if err != nil {
		return nil, err
	}

	return s.newWebAuthnInstance(normalizedOrigin, challenge.SessionData.RelyingPartyID)
}

func (s *AuthService) newWebAuthnInstance(origin string, rpID string) (*webauthn.WebAuthn, error) {
	return webauthn.New(&webauthn.Config{
		RPID:          rpID,
		RPDisplayName: s.webauthnRPDisplayName,
		RPOrigins:     []string{origin},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			UserVerification: protocol.VerificationRequired,
		},
	})
}

func (s *AuthService) normalizeWebAuthnOrigin(origin string) (string, *url.URL, error) {
	normalized, parsed, err := normalizeOriginString(origin)
	if err != nil {
		return "", nil, fmt.Errorf("invalid webauthn origin: %w", err)
	}

	host := parsed.Hostname()
	isLocalHost := host == "localhost" || host == "127.0.0.1"
	if !isLocalHost && parsed.Scheme != "https" {
		return "", nil, fmt.Errorf("passkeys require https outside localhost")
	}

	if len(s.webauthnAllowedOrigins) > 0 {
		matched := false
		for _, allowedOrigin := range s.webauthnAllowedOrigins {
			if normalized == allowedOrigin {
				matched = true
				break
			}
		}
		if !matched {
			return "", nil, fmt.Errorf("origin %s is not allowed for passkeys", normalized)
		}
	}

	return normalized, parsed, nil
}

func normalizeOriginString(origin string) (string, *url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return "", nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", nil, fmt.Errorf("origin must include scheme and host")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		parsed.Path = ""
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	normalized := fmt.Sprintf("%s://%s", strings.ToLower(parsed.Scheme), strings.ToLower(parsed.Host))
	normalizedParsed, err := url.Parse(normalized)
	if err != nil {
		return "", nil, err
	}
	return normalized, normalizedParsed, nil
}

func (s *AuthService) resolveWebAuthnRPID(origin *url.URL) (string, error) {
	host := strings.ToLower(origin.Hostname())
	if host == "" {
		return "", fmt.Errorf("missing origin host")
	}

	if configured := strings.TrimSpace(s.webauthnRPID); configured != "" {
		configured = strings.ToLower(configured)
		if host != configured && !strings.HasSuffix(host, "."+configured) {
			return "", fmt.Errorf("origin host %s does not match configured rp id %s", host, configured)
		}
		return configured, nil
	}

	return host, nil
}

func cloneByteSlice(value []byte) []byte {
	if len(value) == 0 {
		return nil
	}
	cloned := make([]byte, len(value))
	copy(cloned, value)
	return cloned
}

func randomOpaqueBytes(length int) ([]byte, error) {
	value := make([]byte, length)
	if _, err := rand.Read(value); err != nil {
		return nil, err
	}
	return value, nil
}

func bcryptPassword(password string) (string, error) {
	if err := validatePasswordPolicy(password); err != nil {
		return "", err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hashedPassword), nil
}
