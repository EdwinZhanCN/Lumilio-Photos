package icloud

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (r *Client) validateToken() error {
	fmt.Printf("Checking session token validity\n")
	r.debugf(
		"validate session start apple_id=%s session_token=%t trust_token=%t",
		maskAppleID(r.appleID),
		r.sessionData.SessionToken != "",
		r.sessionData.TrustToken != "",
	)

	text, err := r.request(&rawReq{
		Method:  http.MethodPost,
		URL:     r.setupEndpoint + "/validate",
		Headers: r.getCommonHeaders(map[string]string{}),
	})
	if err != nil {
		return fmt.Errorf("validateToken failed, err: %w", err)
	}

	res := new(ValidateData)
	if err = json.Unmarshal([]byte(text), res); err != nil {
		return fmt.Errorf("validateToken unmarshal failed, err: %w, text: %s", err, text)
	}
	r.Data = res
	r.debugState("validate_response")

	return nil
}
