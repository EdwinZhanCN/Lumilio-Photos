package icloud

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// auth using session token
func (r *Client) authWithToken() error {
	r.debugf(
		"accountLogin start apple_id=%s account_country=%q session_token=%t trust_token=%t",
		maskAppleID(r.appleID),
		r.sessionData.AccountCountry,
		r.sessionData.SessionToken != "",
		r.sessionData.TrustToken != "",
	)

	text, err := r.request(&rawReq{
		Method:  http.MethodPost,
		URL:     r.setupEndpoint + "/accountLogin",
		Headers: r.getCommonHeaders(map[string]string{}),
		Body: map[string]any{
			"accountCountryCode": r.sessionData.AccountCountry,
			"dsWebAuthToken":     r.sessionData.SessionToken,
			"extended_login":     true,
			"trustToken":         r.sessionData.TrustToken,
		},
		ExpectStatus: newSet[int](200),
	})
	if err != nil {
		return fmt.Errorf("authWithToken failed, err: %w", err)
	}

	data := new(ValidateData)
	if err = json.Unmarshal([]byte(text), data); err != nil {
		return fmt.Errorf("authWithToken unmarshal failed, text: %s", text)
	}
	r.Data = data
	r.debugState("account_login_response")
	return nil
}
