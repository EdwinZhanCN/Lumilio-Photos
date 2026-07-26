package dto

import (
	"time"

	"server/internal/service"
)

// LoginRequestDTO represents the request structure for user login
type LoginRequestDTO struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginOptionsRequestDTO is the identifier-first probe before password/passkey UI.
type LoginOptionsRequestDTO struct {
	Username string `json:"username" binding:"required"`
}

// LoginOptionsResponseDTO reports which login methods the client should offer.
// Unknown/inactive usernames intentionally match the password-only shape.
// TOTP is never included — it is revealed only after a successful password check.
type LoginOptionsResponseDTO struct {
	Password bool `json:"password" example:"true"`
	Passkey  bool `json:"passkey" example:"false"`
}

type BrowserCapabilitiesDTO struct {
	PrimaryOrigin            string `json:"primary_origin" example:"http://localhost:6680"`
	CurrentOrigin            string `json:"current_origin" example:"http://192.168.1.20:6680"`
	PasskeyAvailable         bool   `json:"passkey_available" example:"false"`
	PasskeyUnavailableReason string `json:"passkey_unavailable_reason,omitempty" enums:"disabled,secure_origin_required,non_primary_origin,trusted_proxy_required,invalid_request_origin"`
	InsecureTransport        bool   `json:"insecure_transport" example:"true"`
	ProxyRequired            bool   `json:"proxy_required" example:"false"`
	ViaTrustedProxy          bool   `json:"via_trusted_proxy" example:"false"`
}

func ToLoginOptionsResponseDTO(options service.LoginOptions) LoginOptionsResponseDTO {
	return LoginOptionsResponseDTO{
		Password: options.Password,
		Passkey:  options.Passkey,
	}
}

type CompleteRequiredPasswordChangeRequestDTO struct {
	PasswordChangeToken string `json:"password_change_token" binding:"required"`
	NewPassword         string `json:"new_password" binding:"required"`
}

// UserDTO represents user information
type UserDTO struct {
	UserID        int        `json:"user_id"`
	Username      string     `json:"username"`
	DisplayName   string     `json:"display_name"`
	AvatarAssetID *string    `json:"avatar_asset_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	IsActive      bool       `json:"is_active"`
	LastLogin     *time.Time `json:"last_login,omitempty"`
	Role          string     `json:"role"`
	Permissions   []string   `json:"permissions"`
}

// AuthResponseDTO represents the response structure for authentication operations
type AuthResponseDTO struct {
	User                   *UserDTO   `json:"user,omitempty"`
	AccessToken            string     `json:"token,omitempty"`
	CSRFToken              string     `json:"csrfToken,omitempty"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty"`
	RequiresMFA            bool       `json:"requires_mfa"`
	MFAToken               string     `json:"mfa_token,omitempty"`
	MFAMethods             []string   `json:"mfa_methods,omitempty"`
	BootstrapAdmin         bool       `json:"bootstrap_admin,omitempty"`
	RequiresPasswordChange bool       `json:"requires_password_change"`
	PasswordChangeToken    string     `json:"password_change_token,omitempty"`
}

type CSRFTokenDTO struct {
	CSRFToken string `json:"csrfToken"`
}

type MediaTokenDTO struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

func ToAuthResponseDTO(response *service.AuthResponse) *AuthResponseDTO {
	if response == nil {
		return nil
	}

	var user *UserDTO
	if response.User != nil {
		dtoUser := ToUserDTO(*response.User)
		user = &dtoUser
	}

	return &AuthResponseDTO{
		User:                   user,
		AccessToken:            response.AccessToken,
		ExpiresAt:              response.ExpiresAt,
		RequiresMFA:            response.RequiresMFA,
		MFAToken:               response.MFAToken,
		MFAMethods:             append([]string(nil), response.MFAMethods...),
		BootstrapAdmin:         response.BootstrapAdmin,
		RequiresPasswordChange: response.RequiresPasswordChange,
		PasswordChangeToken:    response.PasswordChangeToken,
	}
}
