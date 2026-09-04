package dto

import (
	"time"

	"server/internal/service"
)

type RegistrationStartRequestDTO struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=6"`
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

type PasskeyEnrollmentVerifyRequestDTO struct {
	ChallengeToken string `json:"challenge_token" binding:"required"`
	Credential     any    `json:"credential" binding:"required"`
	SecurityToken  string `json:"security_token" binding:"required"`
}

type PasskeyDeleteRequestDTO struct {
	SecurityToken string `json:"security_token" binding:"required"`
}

type PasskeyEnrollmentResponseDTO struct {
	Credential PasskeyCredentialSummaryDTO `json:"credential"`
	Session    *AuthResponseDTO            `json:"session,omitempty"`
}

type PasskeyMutationResponseDTO struct {
	Status  MFAStatusDTO     `json:"status"`
	Session *AuthResponseDTO `json:"session,omitempty"`
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

func ToPasskeyOptionsResponseDTO(response service.PasskeyOptionsResponse) PasskeyOptionsResponseDTO {
	return PasskeyOptionsResponseDTO{
		Options:        response.Options,
		ChallengeToken: response.ChallengeToken,
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

func ToPasskeyEnrollmentResponseDTO(response service.PasskeyEnrollmentResponse) PasskeyEnrollmentResponseDTO {
	var session *AuthResponseDTO
	if response.Session != nil {
		session = ToAuthResponseDTO(response.Session)
	}
	return PasskeyEnrollmentResponseDTO{
		Credential: ToPasskeyCredentialSummaryDTO(response.Credential),
		Session:    session,
	}
}

func ToPasskeyMutationResponseDTO(response service.MFAStatusResponse) PasskeyMutationResponseDTO {
	var session *AuthResponseDTO
	if response.Session != nil {
		session = ToAuthResponseDTO(response.Session)
	}
	return PasskeyMutationResponseDTO{Status: ToMFAStatusDTO(response.Status), Session: session}
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
