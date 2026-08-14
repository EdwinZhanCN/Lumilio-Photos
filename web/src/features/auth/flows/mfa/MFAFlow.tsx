import React from "react";
import { KeyRound, Lock, ShieldCheck, Smartphone } from "lucide-react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import {
  AuthShell,
  Btn,
  CardHead,
  InlineError,
  PasswordField,
  RecoveryCodesPanel,
  TextInput,
  TotpSetupPanel,
} from "../../components/ui";
import { useMFAFlow } from "./useMFAFlow.ts";

export default function MFAFlow(): React.ReactNode {
  const { t } = useI18n();
  const {
    statusQuery,
    status,
    setupResponse,
    verificationCode,
    setVerificationCode,
    password,
    setPassword,
    securityCode,
    setSecurityCode,
    securityMethod,
    setSecurityMethod,
    error,
    recoveryCodes,
    activeAction,
    setActiveAction,
    backTo,
    resetAction,
    handleBeginSetup,
    handleEnable,
    handleDisable,
    handleRegenerate,
    finishRecoveryCodes,
    isBeginningSetup,
    isEnabling,
    isDisabling,
    isRegenerating,
  } = useMFAFlow();

  const backLink = (label: string) => (
    <Link
      to={backTo}
      className="mx-auto text-sm font-medium text-base-content/55 underline-offset-2 hover:text-base-content hover:underline"
    >
      {label}
    </Link>
  );

  let body: React.ReactNode;

  if (statusQuery.isLoading) {
    body = (
      <div className="flex items-center justify-center gap-3 py-8 text-base-content/55">
        <span className="loading loading-spinner loading-sm" />
        {t("common.loading", { defaultValue: "Loading..." })}
      </div>
    );
  } else if (recoveryCodes.length > 0) {
    body = (
      <>
        <CardHead
          icon={KeyRound}
          tone="warning"
          title={t("settings.account.mfa.recoveryCodesTitle", {
            defaultValue: "Save your recovery codes",
          })}
          sub={t("auth.mfa.recoverySubtitle", {
            defaultValue: "Your last resort if you lose your authenticator.",
          })}
        />
        <RecoveryCodesPanel
          codes={recoveryCodes}
          confirmLabel={t("common.done", { defaultValue: "Done" })}
          checkboxLabel={t("auth.register.recoverySavedConfirm", {
            defaultValue: "I’ve saved my recovery codes somewhere safe",
          })}
          onConfirm={finishRecoveryCodes}
        />
      </>
    );
  } else if (activeAction === "setup") {
    body = (
      <>
        <CardHead
          icon={Lock}
          tone="primary"
          title={t("auth.mfa.confirmSetup", {
            defaultValue: "Confirm account security",
          })}
          sub={t("auth.mfa.confirmSetupSubtitle", {
            defaultValue: "Enter your current password before creating an authenticator secret.",
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <form
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault();
            void handleBeginSetup();
          }}
        >
          <PasswordField
            label={t("settings.account.mfa.currentPassword", {
              defaultValue: "Current password",
            })}
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
          />
          {status?.totp_enabled && (
            <>
              <label className="flex flex-col gap-1 text-sm">
                <span className="text-base-content/70">
                  {t("auth.mfa.existingFactor", { defaultValue: "Existing factor" })}
                </span>
                <select
                  className="select select-bordered w-full"
                  value={securityMethod}
                  onChange={(event) =>
                    setSecurityMethod(event.target.value as "totp" | "recovery_code")
                  }
                >
                  <option value="totp">
                    {t("auth.mfa.totpCode", { defaultValue: "Authenticator code" })}
                  </option>
                  <option value="recovery_code">
                    {t("auth.mfa.recoveryCode", { defaultValue: "Recovery code" })}
                  </option>
                </select>
              </label>
              <TextInput
                value={securityCode}
                onChange={(event) => setSecurityCode(event.target.value)}
                inputMode="numeric"
                autoComplete="one-time-code"
                required
              />
            </>
          )}
          <Btn type="submit" variant="primary" loading={isBeginningSetup}>
            {t("auth.mfa.continueSetup", { defaultValue: "Continue" })}
          </Btn>
          <Btn type="button" variant="ghost" onClick={resetAction}>
            {t("common.cancel", { defaultValue: "Cancel" })}
          </Btn>
        </form>
      </>
    );
  } else if (setupResponse) {
    body = (
      <>
        <CardHead
          icon={Smartphone}
          tone="primary"
          title={t("auth.mfa.setupTitle", {
            defaultValue: "Add your authenticator",
          })}
          sub={t("auth.mfa.setupSubtitle", {
            defaultValue: "Scan the code with your authenticator app.",
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <TotpSetupPanel
          otpauthUri={setupResponse.otpauth_uri ?? ""}
          secret={setupResponse.secret ?? ""}
          code={verificationCode}
          onCodeChange={setVerificationCode}
          onVerify={() => void handleEnable()}
          invalid={Boolean(error)}
          busy={isEnabling}
          verifyLabel={t("settings.account.mfa.enableButton", {
            defaultValue: "Verify & enable",
          })}
        />
        {backLink(t("common.cancel", { defaultValue: "Cancel" }))}
      </>
    );
  } else if (activeAction === "disable") {
    body = (
      <>
        <CardHead
          icon={Lock}
          tone="warning"
          title={t("settings.account.mfa.confirmDisable", {
            defaultValue: "Disable two-factor",
          })}
          sub={t("auth.mfa.disableSubtitle", {
            defaultValue:
              "This turns off all MFA — any passkeys on your account will be removed too. Confirm with your password.",
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <form className="flex flex-col gap-4" onSubmit={handleDisable}>
          <PasswordField
            label={t("settings.account.mfa.currentPassword", {
              defaultValue: "Current password",
            })}
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
          />
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-base-content/70">
              {t("auth.mfa.existingFactor", { defaultValue: "Existing factor" })}
            </span>
            <select
              className="select select-bordered w-full"
              value={securityMethod}
              onChange={(event) =>
                setSecurityMethod(event.target.value as "totp" | "recovery_code")
              }
            >
              <option value="totp">
                {t("auth.mfa.totpCode", { defaultValue: "Authenticator code" })}
              </option>
              <option value="recovery_code">
                {t("auth.mfa.recoveryCode", { defaultValue: "Recovery code" })}
              </option>
            </select>
          </label>
          {status?.totp_enabled && (
            <TextInput
              value={securityCode}
              onChange={(event) => setSecurityCode(event.target.value)}
              inputMode="numeric"
              autoComplete="one-time-code"
              placeholder={t("auth.mfa.existingFactorCode", {
                defaultValue: "Current authenticator or recovery code",
              })}
              required
            />
          )}
          <Btn type="submit" variant="primary" loading={isDisabling}>
            {t("settings.account.mfa.confirmDisable", {
              defaultValue: "Disable two-factor",
            })}
          </Btn>
          <Btn type="button" variant="ghost" onClick={resetAction}>
            {t("common.cancel", { defaultValue: "Cancel" })}
          </Btn>
        </form>
      </>
    );
  } else if (activeAction === "regenerate") {
    body = (
      <>
        <CardHead
          icon={KeyRound}
          tone="primary"
          title={t("settings.account.mfa.regenerateTitle", {
            defaultValue: "Regenerate recovery codes",
          })}
          sub={t("auth.mfa.regenerateSubtitle", {
            defaultValue: "This invalidates your existing codes.",
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <form className="flex flex-col gap-4" onSubmit={handleRegenerate}>
          <PasswordField
            label={t("settings.account.mfa.currentPassword", {
              defaultValue: "Current password",
            })}
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
          />
          <label className="flex flex-col gap-1 text-sm">
            <span className="text-base-content/70">
              {t("auth.mfa.existingFactor", { defaultValue: "Existing factor" })}
            </span>
            <select
              className="select select-bordered w-full"
              value={securityMethod}
              onChange={(event) =>
                setSecurityMethod(event.target.value as "totp" | "recovery_code")
              }
            >
              <option value="totp">
                {t("auth.mfa.totpCode", { defaultValue: "Authenticator code" })}
              </option>
              <option value="recovery_code">
                {t("auth.mfa.recoveryCode", { defaultValue: "Recovery code" })}
              </option>
            </select>
          </label>
          <TextInput
            value={securityCode}
            onChange={(event) => setSecurityCode(event.target.value)}
            inputMode="numeric"
            autoComplete="one-time-code"
            placeholder={t("auth.mfa.existingFactorCode", {
              defaultValue: "Current authenticator or recovery code",
            })}
            required
          />
          <Btn type="submit" variant="primary" loading={isRegenerating}>
            {t("settings.account.mfa.confirmRegenerate", {
              defaultValue: "Generate new codes",
            })}
          </Btn>
          <Btn type="button" variant="ghost" onClick={resetAction}>
            {t("common.cancel", { defaultValue: "Cancel" })}
          </Btn>
        </form>
      </>
    );
  } else if (status?.totp_enabled) {
    body = (
      <>
        <CardHead
          icon={ShieldCheck}
          tone="success"
          title={t("auth.mfa.enabledTitle", {
            defaultValue: "Two-factor is on",
          })}
          sub={t("settings.account.mfa.remainingCodes", {
            defaultValue: "{{count}} recovery codes remaining",
            count: status?.recovery_codes_remaining ?? 0,
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <Btn variant="outline" icon={KeyRound} onClick={() => setActiveAction("regenerate")}>
          {t("settings.account.mfa.regenerateButton", {
            defaultValue: "Regenerate recovery codes",
          })}
        </Btn>
        <Btn variant="ghost" onClick={() => setActiveAction("disable")}>
          {t("settings.account.mfa.disableButton", {
            defaultValue: "Disable two-factor",
          })}
        </Btn>
        {backLink(t("auth.mfa.backToSettings", { defaultValue: "Back to settings" }))}
      </>
    );
  } else {
    body = (
      <>
        <CardHead
          icon={ShieldCheck}
          tone="primary"
          title={t("auth.mfa.introTitle", {
            defaultValue: "Two-factor authentication",
          })}
          sub={t("auth.mfa.introSubtitle", {
            defaultValue: "Protect your account with a TOTP authenticator app.",
          })}
        />
        {error && <InlineError>{error}</InlineError>}
        <Btn
          variant="primary"
          icon={ShieldCheck}
          loading={isBeginningSetup}
          onClick={() => setActiveAction("setup")}
        >
          {t("settings.account.mfa.beginSetup", {
            defaultValue: "Set up two-factor",
          })}
        </Btn>
        {backLink(t("auth.mfa.backToSettings", { defaultValue: "Back to settings" }))}
      </>
    );
  }

  return (
    <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
      <AuthShell width={460} appName={t("app.name")}>
        {body}
      </AuthShell>
    </div>
  );
}
