import React, { useEffect, useMemo, useRef, useState } from "react";
import { KeyRound, Lock, ShieldCheck, Smartphone } from "lucide-react";
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useBeginTOTPSetup,
  useDisableTOTP,
  useEnableTOTP,
  useMFAStatus,
  useRegenerateRecoveryCodes,
  type TOTPSetupResponse,
} from "../api/useMFA.ts";
import {
  AuthShell,
  Btn,
  CardHead,
  InlineError,
  PasswordField,
  RecoveryCodesPanel,
  TotpSetupPanel,
} from "../components/ui";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

type ReturnState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const maybeApiError = error as { message?: string; error?: string };
    if (maybeApiError.message) return maybeApiError.message;
    if (maybeApiError.error) return maybeApiError.error;
  }
  return fallback;
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function MFAPage(): React.ReactNode {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const statusQuery = useMFAStatus();
  const beginSetupMutation = useBeginTOTPSetup();
  const enableTOTP = useEnableTOTP();
  const disableTOTP = useDisableTOTP();
  const regenerateRecoveryCodes = useRegenerateRecoveryCodes();
  const autoSetupTriggeredRef = useRef(false);

  const [setupResponse, setSetupResponse] = useState<TOTPSetupResponse | null>(null);
  const [verificationCode, setVerificationCode] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [activeAction, setActiveAction] = useState<"disable" | "regenerate" | null>(null);

  const status = statusQuery.data;
  const shouldAutoStartSetup = searchParams.get("mfa") === "setup";
  const requestedAction = searchParams.get("action");

  const backTo = useMemo(() => {
    const from = (location.state as ReturnState | null)?.from;
    if (!from?.pathname) {
      return "/settings?tab=account";
    }
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const clearFlowParams = (...keys: string[]) => {
    const nextParams = new URLSearchParams(searchParams);
    for (const key of keys) {
      nextParams.delete(key);
    }
    setSearchParams(nextParams, { replace: true });
  };

  const resetAction = () => {
    setActiveAction(null);
    setPassword("");
    setError(null);
    clearFlowParams("action");
  };

  /* ---- handlers ---- */

  const handleBeginSetup = async () => {
    setError(null);
    try {
      const response = await beginSetupMutation.mutateAsync({});
      const payload = response;
      if (payload) {
        setSetupResponse(payload);
        setVerificationCode("");
        setRecoveryCodes([]);
        setActiveAction(null);
      }
    } catch (err) {
      setError(
        getErrorMessage(
          err,
          t("settings.account.mfa.setupError", {
            defaultValue: "Failed to start TOTP setup.",
          }),
        ),
      );
    }
  };

  const handleEnable = async () => {
    const setupToken = setupResponse?.setup_token;
    if (!setupToken || verificationCode.length < 6) return;
    setError(null);
    try {
      const response = await enableTOTP.mutateAsync({
        body: {
          setup_token: setupToken,
          code: verificationCode,
        },
      });
      const payload = response;
      setRecoveryCodes(payload?.recovery_codes ?? []);
      setSetupResponse(null);
      setVerificationCode("");
      clearFlowParams("mfa", "action");
    } catch (err) {
      setError(
        getErrorMessage(
          err,
          t("settings.account.mfa.enableError", {
            defaultValue: "Failed to enable TOTP.",
          }),
        ),
      );
    }
  };

  const handleDisable = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    try {
      await disableTOTP.mutateAsync({ body: { current_password: password } });
      resetAction();
      setRecoveryCodes([]);
    } catch (err) {
      setError(
        getErrorMessage(
          err,
          t("settings.account.mfa.disableError", {
            defaultValue: "Failed to disable TOTP.",
          }),
        ),
      );
    }
  };

  const handleRegenerate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    try {
      const response = await regenerateRecoveryCodes.mutateAsync({
        body: { current_password: password },
      });
      const payload = response;
      setRecoveryCodes(payload?.recovery_codes ?? []);
      setActiveAction(null);
      setPassword("");
      clearFlowParams("action");
    } catch (err) {
      setError(
        getErrorMessage(
          err,
          t("settings.account.mfa.regenerateError", {
            defaultValue: "Failed to regenerate recovery codes.",
          }),
        ),
      );
    }
  };

  /* ---- effects ---- */

  useEffect(() => {
    if (
      !shouldAutoStartSetup ||
      autoSetupTriggeredRef.current ||
      statusQuery.isLoading ||
      status?.totp_enabled ||
      setupResponse ||
      beginSetupMutation.isPending
    ) {
      return;
    }
    autoSetupTriggeredRef.current = true;
    void handleBeginSetup();
  }, [
    beginSetupMutation.isPending,
    setupResponse,
    shouldAutoStartSetup,
    status?.totp_enabled,
    statusQuery.isLoading,
  ]);

  useEffect(() => {
    if (statusQuery.isLoading || !status?.totp_enabled) return;
    if (requestedAction === "disable") setActiveAction("disable");
    else if (requestedAction === "regenerate") setActiveAction("regenerate");
  }, [requestedAction, status?.totp_enabled, statusQuery.isLoading]);

  /* ================================================================ */
  /*  Render                                                           */
  /* ================================================================ */

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
          onConfirm={() => navigate(backTo)}
        />
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
          busy={enableTOTP.isPending}
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
          <Btn type="submit" variant="primary" loading={disableTOTP.isPending}>
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
          <Btn type="submit" variant="primary" loading={regenerateRecoveryCodes.isPending}>
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
          loading={beginSetupMutation.isPending}
          onClick={() => void handleBeginSetup()}
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
