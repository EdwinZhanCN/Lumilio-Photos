package icloud

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// TrustedPhoneNumber represents a phone number that can receive SMS codes.
type TrustedPhoneNumber struct {
	ID                 int    `json:"id"`
	NumberWithDialCode string `json:"numberWithDialCode"`
	PushMode           string `json:"pushMode"`
}

// authOptionsResponse is the response from GET /appleauth/auth containing
// trusted phone numbers and security code metadata.
type authOptionsResponse struct {
	TrustedPhoneNumbers []TrustedPhoneNumber `json:"trustedPhoneNumbers"`
	NoTrustedDevices    bool                 `json:"noTrustedDevices"`
	SecurityCode        *struct {
		Length                int  `json:"length"`
		TooManyCodesSent      bool `json:"tooManyCodesSent"`
		TooManyCodesValidated bool `json:"tooManyCodesValidated"`
		SecurityCodeLocked    bool `json:"securityCodeLocked"`
		SecurityCodeCooldown  bool `json:"securityCodeCooldown"`
	} `json:"securityCode"`
	TwoSV *twoSVResponse `json:"twoSV"`
}

type twoSVResponse struct {
	PhoneNumberVerification *phoneNumberVerification `json:"phoneNumberVerification"`
	BridgeInitiateData      *bridgeInitiateData      `json:"bridgeInitiateData"`
}

type bridgeInitiateData struct {
	PhoneNumberVerification *phoneNumberVerification `json:"phoneNumberVerification"`
}

type phoneNumberVerification struct {
	TrustedPhoneNumbers []TrustedPhoneNumber `json:"trustedPhoneNumbers"`
}

// GetAuthOptions fetches the current auth options including trusted phone numbers.
func (r *Client) GetAuthOptions() (*authOptionsResponse, error) {
	headers := r.getAuthHeaders(map[string]string{
		"Accept": "application/json",
	})

	text, err := r.request(&rawReq{
		Method:  http.MethodGet,
		URL:     r.authEndpoint,
		Headers: headers,
	})
	if err != nil {
		return nil, fmt.Errorf("get auth options failed: %w", err)
	}

	r.debugf("auth options raw response (truncated): %.2000s", text)

	var resp authOptionsResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal auth options failed: %w", err)
	}

	r.debugf(
		"auth options apple_id=%s trusted_phones_top=%d no_trusted_devices=%t two_sv_present=%t",
		maskAppleID(r.appleID),
		len(resp.TrustedPhoneNumbers),
		resp.NoTrustedDevices,
		resp.TwoSV != nil,
	)

	return &resp, nil
}

// GetTrustedPhoneNumbers returns the trusted phone numbers, checking both the
// top-level response and the new bridgeInitiateData path that Apple introduced.
// If Apple does not return phone numbers in any known path, returns a synthetic
// entry with ID=1 as a fallback (most accounts have a single trusted phone).
func (r *Client) GetTrustedPhoneNumbers() ([]TrustedPhoneNumber, error) {
	opts, err := r.GetAuthOptions()
	if err != nil {
		r.debugf("get auth options failed, using fallback phone id=1: %v", err)
		return []TrustedPhoneNumber{{ID: 1, PushMode: "sms"}}, nil
	}

	phones := opts.TrustedPhoneNumbers

	if len(phones) == 0 && opts.TwoSV != nil {
		if opts.TwoSV.PhoneNumberVerification != nil && len(opts.TwoSV.PhoneNumberVerification.TrustedPhoneNumbers) > 0 {
			phones = opts.TwoSV.PhoneNumberVerification.TrustedPhoneNumbers
		}
		if len(phones) == 0 && opts.TwoSV.BridgeInitiateData != nil && opts.TwoSV.BridgeInitiateData.PhoneNumberVerification != nil {
			phones = opts.TwoSV.BridgeInitiateData.PhoneNumberVerification.TrustedPhoneNumbers
		}
	}

	if len(phones) == 0 {
		r.debugf("no trusted phone numbers in auth options response, using fallback phone id=1")
		return []TrustedPhoneNumber{{ID: 1, PushMode: "sms"}}, nil
	}

	r.debugf("resolved trusted phone numbers apple_id=%s count=%d", maskAppleID(r.appleID), len(phones))
	for i, p := range phones {
		r.debugf("  phone[%d] id=%d number=%s push_mode=%s", i, p.ID, p.NumberWithDialCode, p.PushMode)
	}

	return phones, nil
}

// RequestSMSCode sends an SMS verification code to the given phone number.
func (r *Client) RequestSMSCode(phoneID int, mode string) error {
	if mode == "" {
		mode = "sms"
	}

	headers := r.getAuthHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})

	body := map[string]any{
		"phoneNumber": map[string]any{"id": phoneID},
		"mode":        mode,
	}

	r.debugf("requesting sms code apple_id=%s phone_id=%d mode=%s", maskAppleID(r.appleID), phoneID, mode)

	text, status, err := r.requestWithStatus(&rawReq{
		Method:  http.MethodPut,
		URL:     r.authEndpoint + "/verify/phone",
		Headers: headers,
		Body:    body,
	})
	if err != nil {
		r.debugf("request sms code response status=%d body=%.500s err=%v", status, text, err)
		return fmt.Errorf("request SMS code failed: %w", err)
	}

	r.debugf("sms code requested successfully apple_id=%s phone_id=%d status=%d", maskAppleID(r.appleID), phoneID, status)
	return nil
}

// VerifySMSCode submits the SMS verification code received on the phone.
func (r *Client) VerifySMSCode(phoneID int, code string, mode string) error {
	if mode == "" {
		mode = "sms"
	}

	headers := r.getAuthHeaders(map[string]string{
		"Accept":       "application/json",
		"Content-Type": "application/json",
	})

	body := map[string]any{
		"phoneNumber":  map[string]any{"id": phoneID},
		"securityCode": map[string]string{"code": code},
		"mode":         mode,
	}

	r.debugf("verifying sms code apple_id=%s phone_id=%d", maskAppleID(r.appleID), phoneID)

	_, err := r.request(&rawReq{
		Method:       http.MethodPost,
		URL:          r.authEndpoint + "/verify/phone/securitycode",
		Headers:      headers,
		Body:         body,
		ExpectStatus: newSet[int](http.StatusOK, http.StatusNoContent),
	})
	if err != nil {
		if IsErrorCode(err, ErrValidateCodeWrong.Code) {
			return ErrValidateCodeWrong
		}
		return fmt.Errorf("verify SMS code failed: %w", err)
	}

	r.debugf("sms code verified successfully apple_id=%s", maskAppleID(r.appleID))

	if err := r.trustSession(); err != nil {
		return err
	}

	return nil
}

// SignIn performs SRP authentication and accountLogin without attempting 2FA.
// Returns nil if login succeeded, or an error. After a successful SignIn,
// check IsRequires2FA() to determine if verification is needed.
func (r *Client) SignIn(password string) error {
	return r.signIn(password)
}

// IsRequires2FA reports whether the current session requires 2FA verification.
func (r *Client) IsRequires2FA() bool {
	if r.Data == nil || r.Data.DsInfo == nil {
		return false
	}
	return r.isRequires2FA()
}

// Flush persists session data to disk.
func (r *Client) Flush() error {
	return r.flush()
}
