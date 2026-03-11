package dto

import (
	"time"

	"server/internal/service"
)

type RegistrationStartRequestDTO struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegistrationStartResponseDTO struct {
	RegistrationSessionID string `json:"registration_session_id"`
	BootstrapAdmin        bool   `json:"bootstrap_admin"`
	NextRegistrationRole  string `json:"next_registration_role"`
}

type RegistrationSessionRequestDTO struct {
	RegistrationSessionID string `json:"registration_session_id" binding:"required"`
}

type RegistrationPasskeyVerifyRequestDTO struct {
	RegistrationSessionID string `json:"registration_session_id" binding:"required"`
	ChallengeToken        string `json:"challenge_token" binding:"required"`
	Credential            any    `json:"credential" binding:"required"`
}

type RegistrationTOTPCompleteRequestDTO struct {
	RegistrationSessionID string `json:"registration_session_id" binding:"required"`
	Code                  string `json:"code" binding:"required"`
}

type RegistrationTOTPSetupResponseDTO struct {
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
	OtpAuthURI  string `json:"otpauth_uri"`
}

type RegistrationTOTPCompleteResponseDTO struct {
	Auth          *AuthResponseDTO `json:"auth"`
	RecoveryCodes []string         `json:"recovery_codes"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

type PasskeyOptionsRequestDTO struct {
	Username string `json:"username,omitempty"`
}

type PasskeyOptionsResponseDTO struct {
	Options        any    `json:"options"`
	ChallengeToken string `json:"challenge_token"`
}

type PasskeyVerifyRequestDTO struct {
	ChallengeToken string `json:"challenge_token" binding:"required"`
	Credential     any    `json:"credential" binding:"required"`
}

type PasskeyCredentialSummaryDTO struct {
	PasskeyID  int        `json:"passkey_id"`
	Label      string     `json:"label"`
	Transports []string   `json:"transports,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type PasskeyListResponseDTO struct {
	Credentials []PasskeyCredentialSummaryDTO `json:"credentials"`
	Total       int                           `json:"total"`
}

func ToRegistrationStartResponseDTO(response service.RegistrationStartResponse) RegistrationStartResponseDTO {
	return RegistrationStartResponseDTO{
		RegistrationSessionID: response.RegistrationSessionID,
		BootstrapAdmin:        response.BootstrapAdmin,
		NextRegistrationRole:  response.NextRegistrationRole,
	}
}

func ToPasskeyOptionsResponseDTO(response service.PasskeyOptionsResponse) PasskeyOptionsResponseDTO {
	return PasskeyOptionsResponseDTO{
		Options:        response.Options,
		ChallengeToken: response.ChallengeToken,
	}
}

func ToRegistrationTOTPSetupResponseDTO(response service.RegistrationTOTPSetupResponse) RegistrationTOTPSetupResponseDTO {
	return RegistrationTOTPSetupResponseDTO{
		Secret:      response.Secret,
		Issuer:      response.Issuer,
		AccountName: response.AccountName,
		OtpAuthURI:  response.OtpAuthURI,
	}
}

func ToRegistrationTOTPCompleteResponseDTO(response service.RegistrationTOTPCompleteResponse) RegistrationTOTPCompleteResponseDTO {
	return RegistrationTOTPCompleteResponseDTO{
		Auth:          ToAuthResponseDTO(response.AuthResponse),
		RecoveryCodes: append([]string(nil), response.RecoveryCodes...),
		GeneratedAt:   response.GeneratedAt,
	}
}

func ToPasskeyCredentialSummaryDTO(summary service.PasskeyCredentialSummary) PasskeyCredentialSummaryDTO {
	return PasskeyCredentialSummaryDTO{
		PasskeyID:  summary.PasskeyID,
		Label:      summary.Label,
		Transports: append([]string(nil), summary.Transports...),
		CreatedAt:  summary.CreatedAt,
		LastUsedAt: summary.LastUsedAt,
	}
}

func ToPasskeyListResponseDTO(response service.PasskeyListResponse) PasskeyListResponseDTO {
	items := make([]PasskeyCredentialSummaryDTO, 0, len(response.Credentials))
	for _, credential := range response.Credentials {
		items = append(items, ToPasskeyCredentialSummaryDTO(credential))
	}

	return PasskeyListResponseDTO{
		Credentials: items,
		Total:       response.Total,
	}
}
