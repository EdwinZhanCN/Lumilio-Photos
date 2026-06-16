package icloud

import (
	"fmt"
	"strings"
)

func (r *Client) Authenticate(forceRefresh bool, service *string) (finalErr error) {
	defer func() {
		if finalErr == nil {
			r.flush()
		}
	}()

	r.debugf(
		"authenticate start apple_id=%s force_refresh=%t service=%q session_token=%t trust_token=%t",
		maskAppleID(r.appleID),
		forceRefresh,
		stringValue(service),
		r.sessionData != nil && r.sessionData.SessionToken != "",
		r.sessionData != nil && r.sessionData.TrustToken != "",
	)

	var errs []string
	if r.sessionData.SessionToken != "" && !forceRefresh {
		fmt.Printf("Checking session token validity")
		if err := r.validateToken(); err == nil {
			r.debugState("authenticate_cached_session_valid")
			return nil
		} else {
			errs = append(errs, err.Error())
			r.debugf("cached session token invalid apple_id=%s err=%v", maskAppleID(r.appleID), err)
			fmt.Printf("Invalid session token. Attempting brand new login.\n")
		}
	}

	if service != nil {
		if r.Data != nil && len(r.Data.Apps) > 0 && r.Data.Apps[*service] != nil && r.Data.Apps[*service].CanLaunchWithOneFactor {
			fmt.Printf("Authenticating as %s for %s\n", r.appleID, *service)
			if err := r.authWithCredentialsService(*service, r.password); err != nil {
				errs = append(errs, err.Error())
				r.debugf("service auth failed apple_id=%s service=%q err=%v", maskAppleID(r.appleID), *service, err)
				fmt.Printf("Could not log into service. Attempting brand new login.\n")
			} else {
				r.debugState("authenticate_service_login_success")
				return nil
			}
		}
	}

	// default, login to icloud.com[.cn]
	{
		fmt.Printf("Authenticating as %s\n", r.appleID)
		err := r.signIn(r.password)
		if err == nil {
			r.debugState("authenticate_signin_success")
			err = r.verify2Fa()
			if err == nil {
				r.debugState("authenticate_complete")
				return nil
			}
		}
		// self._webservices = self.data["webservices"]
		errs = append(errs, err.Error())
		r.debugf("interactive login failed apple_id=%s err=%v", maskAppleID(r.appleID), err)
		fmt.Printf("Login failed\n")
	}

	return fmt.Errorf("login failed: %s", strings.Join(errs, "; "))
}
