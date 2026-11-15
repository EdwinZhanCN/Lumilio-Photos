package icloud

import (
	"fmt"
	"net/http"
)

// session trust to avoid user log in going forward
func (r *Client) trustSession() error {
	headers := r.getAuthHeaders(map[string]string{})
	r.debugf(
		"trust session start apple_id=%s session_token=%t trust_token=%t",
		maskAppleID(r.appleID),
		r.sessionData.SessionToken != "",
		r.sessionData.TrustToken != "",
	)

	_, err := r.request(&rawReq{
		Method:       http.MethodGet,
		URL:          r.authEndpoint + "/2sv/trust",
		Headers:      headers,
		ExpectStatus: newSet[int](http.StatusNoContent),
	})
	if err != nil {
		r.debugf("trust session failed apple_id=%s err=%v", maskAppleID(r.appleID), err)
		return fmt.Errorf("trustSession failed: %w", err)
	}
	r.debugf("trust session succeeded apple_id=%s", maskAppleID(r.appleID))

	return r.authWithToken()
}
