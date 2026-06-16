package icloud

import (
	"log"
	"strings"
)

func (r *Client) debugf(format string, args ...any) {
	log.Printf("[icloud] "+format, args...)
}

func (r *Client) debugState(stage string) {
	if r == nil {
		log.Printf("[icloud] stage=%s client=nil", stage)
		return
	}

	if r.sessionData == nil {
		log.Printf("[icloud] stage=%s apple_id=%s session_data=nil", stage, maskAppleID(r.appleID))
		return
	}

	fields := []any{
		stage,
		maskAppleID(r.appleID),
		r.sessionData.SessionToken != "",
		r.sessionData.TrustToken != "",
		r.sessionData.Scnt != "",
		r.sessionData.SessionID != "",
		r.sessionData.AccountCountry,
	}

	if r.Data == nil {
		r.debugf("stage=%s apple_id=%s session_token=%t trust_token=%t scnt=%t session_id=%t account_country=%q data=nil", fields...)
		return
	}

	if r.Data.DsInfo == nil {
		r.debugf(
			"stage=%s apple_id=%s session_token=%t trust_token=%t scnt=%t session_id=%t account_country=%q data_present=true ds_info=nil hsa_challenge_required=%t trusted_browser=%t terms_update_needed=%t repair_needed=%t notification_id_present=%t continue_on_device_eligible=%d authorize_ui=%t phone_vetting=%t email_vetting=%t",
			append(
				fields,
				r.Data.HsaChallengeRequired,
				r.Data.HsaTrustedBrowser,
				r.Data.TermsUpdateNeeded,
				r.Data.IsRepairNeeded,
				false,
				0,
				r.Data.ConfigBag.Urls.AccountAuthorizeUI != "",
				r.Data.ConfigBag.Urls.VettingUrlForPhone != "",
				r.Data.ConfigBag.Urls.VettingUrlForEmail != "",
			)...,
		)
		return
	}

	r.debugf(
		"stage=%s apple_id=%s session_token=%t trust_token=%t scnt=%t session_id=%t account_country=%q hsa_version=%d hsa_enabled=%t hsa_challenge_required=%t trusted_browser=%t requires_2fa=%t requires_2sa=%t notification_id_present=%t continue_on_device_eligible=%d authorize_ui=%t phone_vetting=%t email_vetting=%t terms_update_needed=%t repair_needed=%t account_locked=%t web_access_allowed=%t has_icloud_qualifying_device=%t ds_status_code=%d",
		append(
			fields,
			r.Data.DsInfo.HsaVersion,
			r.Data.DsInfo.HsaEnabled,
			r.Data.HsaChallengeRequired,
			r.Data.HsaTrustedBrowser,
			r.isRequires2FA(),
			r.isRequires2SA(),
			r.Data.DsInfo.NotificationId != "",
			len(r.Data.DsInfo.ContinueOnDeviceEligibleDeviceInfo),
			r.Data.ConfigBag.Urls.AccountAuthorizeUI != "",
			r.Data.ConfigBag.Urls.VettingUrlForPhone != "",
			r.Data.ConfigBag.Urls.VettingUrlForEmail != "",
			r.Data.TermsUpdateNeeded,
			r.Data.IsRepairNeeded,
			r.Data.DsInfo.Locked,
			r.Data.DsInfo.IsWebAccessAllowed,
			r.Data.DsInfo.HasICloudQualifyingDevice,
			r.Data.DsInfo.StatusCode,
		)...,
	)
}

func maskAppleID(appleID string) string {
	appleID = strings.TrimSpace(appleID)
	if appleID == "" {
		return ""
	}

	at := strings.Index(appleID, "@")
	if at <= 1 {
		return "***"
	}

	local := appleID[:at]
	domain := appleID[at:]
	if len(local) <= 2 {
		return local[:1] + "*" + domain
	}
	return local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:] + domain
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
