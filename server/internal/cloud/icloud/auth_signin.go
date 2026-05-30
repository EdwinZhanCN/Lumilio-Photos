package icloud

import (
	"fmt"
)

func (r *Client) signIn(password string) error {
	if err := r.signInSRP(password); err != nil {
		return fmt.Errorf("signin failed: %w", err)
	}

	return r.authWithToken()
}
