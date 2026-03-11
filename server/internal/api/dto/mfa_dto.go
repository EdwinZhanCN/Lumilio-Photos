package dto

import (
	"time"

	"server/internal/service"
)

type VerifyMFARequestDTO struct {
	MFAToken string `json:"mfa_token" binding:"required"`
	Code     string `json:"code" binding:"required"`
	Method   string `json:"method" binding:"required,oneof=totp recovery_code"`
}

type MFAStatusDTO struct {
	TOTPEnabled              bool       `json:"totp_enabled"`
	PasskeyCount             int        `json:"passkey_count"`
	RecoveryCodesRemaining   int        `json:"recovery_codes_remaining"`
	RecoveryCodesGeneratedAt *time.Time `json:"recovery_codes_generated_at,omitempty"`
	AvailableMethods         []string   `json:"available_methods,omitempty"`
}

type TOTPSetupResponseDTO struct {
	SetupToken  string `json:"setup_token"`
	Secret      string `json:"secret"`
	Issuer      string `json:"issuer"`
	AccountName string `json:"account_name"`
	OtpAuthURI  string `json:"otpauth_uri"`
}

type EnableTOTPRequestDTO struct {
	SetupToken string `json:"setup_token" binding:"required"`
	Code       string `json:"code" binding:"required"`
}

type DisableTOTPRequestDTO struct {
	CurrentPassword string `json:"current_password" binding:"required"`
}

type RegenerateRecoveryCodesRequestDTO struct {
	CurrentPassword string `json:"current_password" binding:"required"`
}

type RecoveryCodesResponseDTO struct {
	RecoveryCodes []string     `json:"recovery_codes"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Status        MFAStatusDTO `json:"status"`
}

func ToMFAStatusDTO(status service.MFAStatus) MFAStatusDTO {
	return MFAStatusDTO{
		TOTPEnabled:              status.TOTPEnabled,
		PasskeyCount:             status.PasskeyCount,
		RecoveryCodesRemaining:   status.RecoveryCodesRemaining,
		RecoveryCodesGeneratedAt: status.RecoveryCodesGeneratedAt,
		AvailableMethods:         append([]string(nil), status.AvailableMethods...),
	}
}

func ToTOTPSetupResponseDTO(response service.TOTPSetupResponse) TOTPSetupResponseDTO {
	return TOTPSetupResponseDTO{
		SetupToken:  response.SetupToken,
		Secret:      response.Secret,
		Issuer:      response.Issuer,
		AccountName: response.AccountName,
		OtpAuthURI:  response.OtpAuthURI,
	}
}

func ToRecoveryCodesResponseDTO(response service.RecoveryCodesResponse) RecoveryCodesResponseDTO {
	return RecoveryCodesResponseDTO{
		RecoveryCodes: append([]string(nil), response.RecoveryCodes...),
		GeneratedAt:   response.GeneratedAt,
		Status:        ToMFAStatusDTO(response.Status),
	}
}
