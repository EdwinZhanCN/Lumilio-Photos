package icloud

import (
	"fmt"
	"os"
)

func (r *Client) verify2Fa() error {
	if r.Data == nil || r.Data.DsInfo == nil {
		return fmt.Errorf("not authenticated validate data")
	}

	r.debugState("verify_2fa_entry")

	if r.isRequires2FA() {
		r.debugf(
			"apple reports trusted-device 2fa required apple_id=%s notification_id_present=%t eligible_devices=%d trusted_browser=%t",
			maskAppleID(r.appleID),
			r.Data.DsInfo.NotificationId != "",
			len(r.Data.DsInfo.ContinueOnDeviceEligibleDeviceInfo),
			r.Data.HsaTrustedBrowser,
		)
		code, err := r.twoFACodeGetter.GetText(r.appleID)
		if err != nil {
			r.debugf("waiting for trusted-device security code apple_id=%s err=%v", maskAppleID(r.appleID), err)
			return fmt.Errorf("get 2fa code failed, err: %w", err)
		}
		if err := r.validate2FACode(code); err != nil {
			r.debugf("trusted-device security code rejected apple_id=%s err=%v", maskAppleID(r.appleID), err)
			return err
		}

		if !r.Data.HsaTrustedBrowser {
			r.debugf("session is not yet trusted apple_id=%s requesting trust session", maskAppleID(r.appleID))
			if err := r.trustSession(); err != nil {
				return err
			}
		}
		r.debugState("verify_2fa_complete")
	} else if r.isRequires2SA() {
		r.debugf("apple reports legacy 2sa flow apple_id=%s", maskAppleID(r.appleID))
		fmt.Printf("Two-step authentication required. Your trusted devices are:\n")
		devices, err := r.trustedDevices()
		if err != nil {
			return err
		}
		r.debugf("legacy 2sa trusted devices listed apple_id=%s count=%d", maskAppleID(r.appleID), len(devices))
		for i, device := range devices {
			fmt.Printf("  %d: %s\n", i, device.GetName())
		}

		fmt.Printf("not impl")
		os.Exit(1)
	} else {
		r.debugf(
			"apple did not require 2fa apple_id=%s trusted_browser=%t notification_id_present=%t eligible_devices=%d",
			maskAppleID(r.appleID),
			r.Data.HsaTrustedBrowser,
			r.Data.DsInfo.NotificationId != "",
			len(r.Data.DsInfo.ContinueOnDeviceEligibleDeviceInfo),
		)
	}
	return nil
}

func (r *Client) isRequires2FA() bool {
	return r.Data.DsInfo.HsaVersion == 2 && (r.Data.HsaChallengeRequired || !r.Data.HsaTrustedBrowser)
}

func (r *Client) isRequires2SA() bool {
	return r.Data.DsInfo.HsaVersion >= 1 && (r.Data.HsaChallengeRequired || !r.Data.HsaTrustedBrowser)
}
