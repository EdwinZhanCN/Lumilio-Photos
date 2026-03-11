import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  CheckCircle2,
  ChevronDown,
  ClipboardCopy,
  Info,
  KeyRound,
  ShieldCheck,
} from "lucide-react";
import { Link, useLocation, useSearchParams } from "react-router-dom";
import QRCode from "qrcode";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useBeginTOTPSetup,
  useDisableTOTP,
  useEnableTOTP,
  useMFAStatus,
  useRegenerateRecoveryCodes,
  type ApiResult,
  type MFAStatus,
  type RecoveryCodesResponse,
  type TOTPSetupResponse,
} from "../hooks/useMFA.ts";

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

type FeedbackState = {
  tone: "success" | "error";
  message: string;
} | null;

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

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function CopyableField({
  label,
  value,
  mono = false,
  copyLabel,
  hint,
}: {
  label: string;
  value: string;
  mono?: boolean;
  copyLabel: string;
  hint?: string;
}) {
  return (
    <fieldset className="fieldset">
      {label && (
        <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
          {label}
        </legend>
      )}
      <label className="input input-bordered flex w-full items-center gap-2">
        <input
          type="text"
          className={`min-w-0 grow ${mono ? "font-mono text-sm" : ""}`}
          value={value}
          readOnly
        />
        <button
          type="button"
          className="btn btn-ghost btn-xs gap-1 text-base-content/60 hover:text-base-content"
          onClick={() => copyToClipboard(value)}
        >
          <ClipboardCopy className="h-3.5 w-3.5" />
          {copyLabel}
        </button>
      </label>
      {hint && <p className="mt-1 text-xs text-base-content/60">{hint}</p>}
    </fieldset>
  );
}

/* ------------------------------------------------------------------ */
/*  Page                                                               */
/* ------------------------------------------------------------------ */

export default function MFAPage(): React.ReactNode {
  const { t } = useI18n();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const statusQuery = useMFAStatus();
  const beginSetupMutation = useBeginTOTPSetup();
  const enableTOTP = useEnableTOTP();
  const disableTOTP = useDisableTOTP();
  const regenerateRecoveryCodes = useRegenerateRecoveryCodes();
  const autoSetupTriggeredRef = useRef(false);

  const [setupResponse, setSetupResponse] = useState<TOTPSetupResponse | null>(
    null,
  );
  const [verificationCode, setVerificationCode] = useState("");
  const [disablePassword, setDisablePassword] = useState("");
  const [regeneratePassword, setRegeneratePassword] = useState("");
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [revealedRecoveryCodes, setRevealedRecoveryCodes] = useState<string[]>(
    [],
  );
  const [activeAction, setActiveAction] = useState<
    "disable" | "regenerate" | null
  >(null);
  const [qrCodeDataURL, setQrCodeDataURL] = useState<string | null>(null);

  const status = statusQuery.data?.data as MFAStatus | undefined;
  const isBusy =
    beginSetupMutation.isPending ||
    enableTOTP.isPending ||
    disableTOTP.isPending ||
    regenerateRecoveryCodes.isPending;
  const recoveryCodesText = useMemo(
    () => revealedRecoveryCodes.join("\n"),
    [revealedRecoveryCodes],
  );
  const isBootstrapOnboarding =
    searchParams.get("welcome") === "bootstrap-admin";
  const shouldAutoStartSetup = searchParams.get("mfa") === "setup";
  const requestedAction = searchParams.get("action");

  const backTo = useMemo(() => {
    const from = (location.state as ReturnState | null)?.from;
    if (!from?.pathname) {
      return isBootstrapOnboarding ? "/" : "/settings?tab=account";
    }
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [isBootstrapOnboarding, location.state]);

  const clearFlowParams = (...keys: string[]) => {
    const nextParams = new URLSearchParams(searchParams);
    for (const key of keys) {
      nextParams.delete(key);
    }
    setSearchParams(nextParams, { replace: true });
  };

  /* ---- handlers ---- */

  const handleBeginSetup = async () => {
    setFeedback(null);

    try {
      const response = await beginSetupMutation.mutateAsync({});
      const payload = response as ApiResult<TOTPSetupResponse> | undefined;
      if (payload?.data) {
        setSetupResponse(payload.data);
        setVerificationCode("");
        setRevealedRecoveryCodes([]);
        setActiveAction(null);
      }
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.account.mfa.setupError", {
            defaultValue: "Failed to start TOTP setup.",
          }),
        ),
      });
    }
  };

  const handleEnable = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!setupResponse) return;

    setFeedback(null);

    try {
      const response = await enableTOTP.mutateAsync({
        body: {
          setup_token: setupResponse.setup_token,
          code: verificationCode,
        },
      });
      const payload = response as ApiResult<RecoveryCodesResponse> | undefined;
      if (payload?.data) {
        setRevealedRecoveryCodes(payload.data.recovery_codes ?? []);
      }
      setSetupResponse(null);
      setVerificationCode("");
      setQrCodeDataURL(null);
      if (shouldAutoStartSetup || isBootstrapOnboarding) {
        clearFlowParams("mfa", "welcome", "action");
      }
      setFeedback({
        tone: "success",
        message: t("settings.account.mfa.enabled", {
          defaultValue: "Two-factor authentication is now enabled.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.account.mfa.enableError", {
            defaultValue: "Failed to enable TOTP.",
          }),
        ),
      });
    }
  };

  const handleDisable = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);

    try {
      await disableTOTP.mutateAsync({
        body: {
          current_password: disablePassword,
        },
      });
      setDisablePassword("");
      setActiveAction(null);
      setRevealedRecoveryCodes([]);
      clearFlowParams("action");
      setFeedback({
        tone: "success",
        message: t("settings.account.mfa.disabled", {
          defaultValue: "Two-factor authentication has been disabled.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.account.mfa.disableError", {
            defaultValue: "Failed to disable TOTP.",
          }),
        ),
      });
    }
  };

  const handleRegenerate = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);

    try {
      const response = await regenerateRecoveryCodes.mutateAsync({
        body: {
          current_password: regeneratePassword,
        },
      });
      const payload = response as ApiResult<RecoveryCodesResponse> | undefined;
      if (payload?.data) {
        setRevealedRecoveryCodes(payload.data.recovery_codes ?? []);
      }
      setRegeneratePassword("");
      setActiveAction(null);
      clearFlowParams("action");
      setFeedback({
        tone: "success",
        message: t("settings.account.mfa.regenerated", {
          defaultValue: "Recovery codes regenerated successfully.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.account.mfa.regenerateError", {
            defaultValue: "Failed to regenerate recovery codes.",
          }),
        ),
      });
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
    if (statusQuery.isLoading || !status?.totp_enabled) {
      return;
    }

    if (requestedAction === "disable") {
      setActiveAction("disable");
      return;
    }

    if (requestedAction === "regenerate") {
      setActiveAction("regenerate");
    }
  }, [requestedAction, status?.totp_enabled, statusQuery.isLoading]);

  useEffect(() => {
    let cancelled = false;

    const otpauthURI = setupResponse?.otpauth_uri;
    if (!otpauthURI) {
      setQrCodeDataURL(null);
      return undefined;
    }

    QRCode.toDataURL(otpauthURI, {
      errorCorrectionLevel: "M",
      margin: 1,
      width: 224,
      color: {
        dark: "#111827",
        light: "#ffffff",
      },
    })
      .then((dataURL: string) => {
        if (!cancelled) {
          setQrCodeDataURL(dataURL);
        }
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        setQrCodeDataURL(null);
        setFeedback({
          tone: "error",
          message: getErrorMessage(
            error,
            t("settings.account.mfa.qrError", {
              defaultValue: "Failed to generate the TOTP QR code.",
            }),
          ),
        });
      });

    return () => {
      cancelled = true;
    };
  }, [setupResponse?.otpauth_uri, t]);

  /* ================================================================ */
  /*  Render                                                           */
  /* ================================================================ */

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-base-200 px-4 py-12">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -right-48 -top-48 h-96 w-96 rounded-full bg-primary/8 blur-3xl" />
        <div className="absolute -bottom-48 -left-48 h-96 w-96 rounded-full bg-secondary/8 blur-3xl" />
      </div>

      <div className="relative w-full max-w-2xl">
        <div className="card bg-base-100 shadow-2xl ring-1 ring-base-content/5">
          <div className="card-body gap-7 p-8 sm:p-10">
            {/* ── Header ─────────────────────────────────────────── */}
            <div className="flex flex-col items-center gap-5 text-center">
              <div className="inline-flex items-center gap-4 text-3xl font-bold tracking-tight text-base-content sm:text-4xl">
                <img
                  src="/logo.png"
                  alt={t("app.name") + " Logo"}
                  className="size-10 bg-contain object-contain sm:size-12"
                />
                <span>{t("app.name")}</span>
              </div>

              <div className="space-y-1.5">
                <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
                  {isBootstrapOnboarding
                    ? t("auth.mfa.bootstrapTitle", {
                        defaultValue: "Protect the first Admin account",
                      })
                    : t("auth.mfa.pageTitle", {
                        defaultValue: "Manage your authenticator setup",
                      })}
                </h1>

                <p className="text-sm text-base-content/80">
                  {isBootstrapOnboarding
                    ? t("auth.mfa.bootstrapSubtitle", {
                        defaultValue:
                          "Finish TOTP setup now so the first Lumilio Admin account is protected before you continue.",
                      })
                    : t("auth.mfa.pageSubtitle", {
                        defaultValue:
                          "Use a TOTP app to manage sign-in verification, recovery codes, and account security from one dedicated page.",
                      })}
                </p>
              </div>
            </div>

            {/* ── Bootstrap onboarding warning ───────────────────── */}
            {isBootstrapOnboarding && !status?.totp_enabled && (
              <div className="rounded-xl border border-primary/40 bg-primary/10 p-4">
                <p className="text-sm font-semibold text-primary">
                  {t("settings.account.mfa.bootstrapOnboardingTitle", {
                    defaultValue: "First Admin account",
                  })}
                </p>
                <p className="mt-1.5 text-xs text-base-content/80">
                  {t("settings.account.mfa.bootstrapOnboarding", {
                    defaultValue:
                      "This is the first Lumilio Admin account. Finish TOTP setup now so your Admin login starts protected from the first session.",
                  })}
                </p>
              </div>
            )}

            {/* ── Feedback alert ─────────────────────────────────── */}
            {feedback && (
              <div
                className={`alert py-3 text-sm ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
              >
                {feedback.tone === "success" ? (
                  <CheckCircle2 className="h-4 w-4 shrink-0" />
                ) : (
                  <AlertCircle className="h-4 w-4 shrink-0" />
                )}
                <span>{feedback.message}</span>
              </div>
            )}

            {/* ── Status bar ─────────────────────────────────────── */}
            {statusQuery.isLoading ? (
              <div className="flex items-center justify-center gap-3 rounded-xl border border-base-300 bg-base-200/50 p-4">
                <span className="loading loading-spinner loading-sm" />
                <span className="text-sm text-base-content/70">
                  {t("common.loading", { defaultValue: "Loading..." })}
                </span>
              </div>
            ) : (
              <div className="rounded-xl border border-base-300 bg-base-200/50 p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div className="flex flex-wrap items-center gap-3">
                    <span
                      className={`badge font-medium ${
                        status?.totp_enabled
                          ? "badge-success badge-outline"
                          : "badge-ghost"
                      }`}
                    >
                      {status?.totp_enabled
                        ? t("settings.account.mfa.enabledBadge", {
                            defaultValue: "Enabled",
                          })
                        : t("settings.account.mfa.disabledBadge", {
                            defaultValue: "Disabled",
                          })}
                    </span>
                    <span className="text-sm text-base-content/70">
                      {t("settings.account.mfa.remainingCodes", {
                        defaultValue: "{{count}} recovery codes remaining",
                        count: status?.recovery_codes_remaining ?? 0,
                      })}
                    </span>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    {!status?.totp_enabled && !setupResponse && (
                      <button
                        type="button"
                        className={`btn btn-primary btn-sm ${beginSetupMutation.isPending ? "loading" : ""}`}
                        disabled={isBusy}
                        onClick={handleBeginSetup}
                      >
                        {!beginSetupMutation.isPending && (
                          <ShieldCheck className="h-4 w-4" />
                        )}
                        {t("settings.account.mfa.beginSetup", {
                          defaultValue: "Set up TOTP",
                        })}
                      </button>
                    )}

                    {status?.totp_enabled && (
                      <>
                        <button
                          type="button"
                          className={`btn btn-outline btn-sm ${activeAction === "regenerate" ? "btn-active" : ""}`}
                          onClick={() =>
                            setActiveAction((c) =>
                              c === "regenerate" ? null : "regenerate",
                            )
                          }
                        >
                          {t("settings.account.mfa.regenerateButton", {
                            defaultValue: "Regenerate recovery codes",
                          })}
                        </button>
                        <button
                          type="button"
                          className={`btn btn-outline btn-error btn-sm ${activeAction === "disable" ? "btn-active" : ""}`}
                          onClick={() =>
                            setActiveAction((c) =>
                              c === "disable" ? null : "disable",
                            )
                          }
                        >
                          {t("settings.account.mfa.disableButton", {
                            defaultValue: "Disable TOTP",
                          })}
                        </button>
                      </>
                    )}
                  </div>
                </div>
              </div>
            )}

            {/* ── Setup flow ─────────────────────────────────────── */}
            {setupResponse && (
              <div className="flex flex-col gap-5">
                {/* STEP 1: Scan QR */}
                <div className="space-y-5">
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase tracking-wider text-primary">
                      {t("settings.account.mfa.stepOne", {
                        defaultValue: "Step 1",
                      })}
                    </p>
                    <h4 className="text-lg font-semibold tracking-tight">
                      {t("settings.account.mfa.setupTitle", {
                        defaultValue:
                          "Scan the QR code with your authenticator app",
                      })}
                    </h4>
                    <p className="text-sm text-base-content/80">
                      {t("settings.account.mfa.setupDescription", {
                        defaultValue:
                          "Use Duo, 1Password, Google Authenticator, or another TOTP-compatible app. If scanning fails, copy the setup details manually.",
                      })}
                    </p>
                  </div>

                  <div className="grid gap-5 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
                    {/* QR column */}
                    <div className="rounded-2xl border border-base-300 bg-base-200/50 p-4">
                      <div className="mx-auto aspect-square w-full max-w-60 rounded-2xl border border-base-300 bg-base-100 p-3 shadow-sm">
                        {qrCodeDataURL ? (
                          <img
                            src={qrCodeDataURL}
                            alt={t("settings.account.mfa.qrAlt", {
                              defaultValue: "TOTP setup QR code",
                            })}
                            className="size-full rounded-xl bg-white object-contain"
                          />
                        ) : (
                          <div className="skeleton size-full rounded-xl" />
                        )}
                      </div>
                      <p className="mt-3 text-center text-xs text-base-content/60">
                        {t("settings.account.mfa.qrCaption", {
                          defaultValue:
                            "If scanning fails, you can still use the manual setup key.",
                        })}
                      </p>
                    </div>

                    {/* Manual setup column */}
                    <div className="flex flex-col gap-2.5">
                      <div className="grid gap-2.5 sm:grid-row-3">
                        <CopyableField
                          label={t("settings.account.mfa.issuer", {
                            defaultValue: "Issuer",
                          })}
                          value={setupResponse.issuer ?? ""}
                          copyLabel={t("common.copy", {
                            defaultValue: "Copy",
                          })}
                        />
                        <CopyableField
                          label={t("settings.account.mfa.account", {
                            defaultValue: "Account",
                          })}
                          value={setupResponse.account_name ?? ""}
                          copyLabel={t("common.copy", {
                            defaultValue: "Copy",
                          })}
                        />
                        <CopyableField
                          label={t("settings.account.mfa.setupKey", {
                            defaultValue: "Manual setup key",
                          })}
                          value={setupResponse.secret ?? ""}
                          mono
                          copyLabel={t("common.copy", {
                            defaultValue: "Copy",
                          })}
                          hint={t("settings.account.mfa.setupKeyHint", {
                            defaultValue:
                              "Enter this key manually in your authenticator app if you can't scan the QR code.",
                          })}
                        />
                      </div>

                      {/* Advanced URI (collapsible) */}
                      <details className="group rounded-xl border border-base-300 bg-base-200/50 open:shadow-sm">
                        <summary className="flex cursor-pointer list-none items-center justify-between gap-4 px-4 py-3 text-xs font-medium uppercase tracking-wider text-base-content/70">
                          <span>
                            {t("settings.account.mfa.advanced", {
                              defaultValue: "Advanced setup URI",
                            })}
                          </span>
                          <ChevronDown className="h-4 w-4 shrink-0 text-base-content/50 transition-transform group-open:rotate-180" />
                        </summary>
                        <div className="border-t border-base-300 px-4 py-4">
                          <CopyableField
                            label=""
                            value={setupResponse.otpauth_uri ?? ""}
                            mono
                            copyLabel={t("common.copy", {
                              defaultValue: "Copy",
                            })}
                          />
                        </div>
                      </details>
                    </div>
                  </div>
                </div>

                <div className="divider my-0" />

                {/* STEP 2: Verify code */}
                <form className="space-y-5" onSubmit={handleEnable}>
                  <div className="space-y-1">
                    <p className="text-xs font-medium uppercase tracking-wider text-primary">
                      {t("settings.account.mfa.stepTwoLabel", {
                        defaultValue: "Step 2",
                      })}
                    </p>
                    <h4 className="text-lg font-semibold tracking-tight">
                      {t("settings.account.mfa.stepTwo", {
                        defaultValue: "Verify with a 6-digit code",
                      })}
                    </h4>
                  </div>

                  <fieldset className="fieldset">
                    <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                      {t("settings.account.mfa.verificationCode", {
                        defaultValue: "Verification code",
                      })}
                    </legend>
                    <label className="input input-bordered validator flex w-full items-center gap-2">
                      <ShieldCheck className="h-4 w-4 shrink-0 text-base-content/70" />
                      <input
                        type="text"
                        inputMode="numeric"
                        placeholder="123456"
                        className="grow font-mono tracking-widest"
                        autoComplete="one-time-code"
                        value={verificationCode}
                        onChange={(event) =>
                          setVerificationCode(event.target.value)
                        }
                        pattern="[0-9]{6}"
                        required
                      />
                      <div
                        className="tooltip tooltip-left cursor-help"
                        data-tip={t("settings.account.mfa.totpValidation", {
                          defaultValue:
                            "Enter the current 6-digit code from your app.",
                        })}
                      >
                        <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                      </div>
                    </label>
                  </fieldset>

                  <div className="flex gap-3">
                    <button
                      type="submit"
                      className={`btn btn-primary w-full ${enableTOTP.isPending ? "loading" : ""}`}
                      disabled={isBusy}
                    >
                      {!enableTOTP.isPending && (
                        <ShieldCheck className="h-4 w-4" />
                      )}
                      {enableTOTP.isPending
                        ? t("settings.account.mfa.enabling", {
                            defaultValue: "Enabling…",
                          })
                        : t("settings.account.mfa.enableButton", {
                            defaultValue: "Enable TOTP",
                          })}
                    </button>
                  </div>
                </form>
              </div>
            )}

            {/* ── Disable TOTP ───────────────────────────────────── */}
            {activeAction === "disable" && status?.totp_enabled && (
              <form className="flex flex-col gap-5" onSubmit={handleDisable}>
                <div className="rounded-xl border border-error/30 bg-error/5 p-4">
                  <p className="text-sm font-semibold text-error">
                    {t("settings.account.mfa.confirmDisable", {
                      defaultValue: "Disable TOTP",
                    })}
                  </p>
                  <p className="mt-1.5 text-xs text-base-content/80">
                    {t("settings.account.mfa.disableWarning", {
                      defaultValue:
                        "This will remove two-factor authentication from your account. Confirm with your current password.",
                    })}
                  </p>
                </div>

                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("settings.account.mfa.currentPassword", {
                      defaultValue: "Current password",
                    })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="password"
                      placeholder="••••••••"
                      className="grow"
                      value={disablePassword}
                      onChange={(event) =>
                        setDisablePassword(event.target.value)
                      }
                      minLength={6}
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={t("settings.account.mfa.currentPasswordHint", {
                        defaultValue: "Confirm with your current password.",
                      })}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <button
                  type="submit"
                  className={`btn btn-error w-full ${disableTOTP.isPending ? "loading" : ""}`}
                  disabled={isBusy}
                >
                  {t("settings.account.mfa.confirmDisable", {
                    defaultValue: "Disable TOTP",
                  })}
                </button>
              </form>
            )}

            {/* ── Regenerate recovery codes ──────────────────────── */}
            {activeAction === "regenerate" && status?.totp_enabled && (
              <form className="flex flex-col gap-5" onSubmit={handleRegenerate}>
                <div className="rounded-xl border border-base-300 bg-base-200/50 p-4">
                  <p className="text-sm font-semibold text-base-content">
                    {t("settings.account.mfa.regenerateTitle", {
                      defaultValue: "Regenerate recovery codes",
                    })}
                  </p>
                  <p className="mt-1.5 text-xs text-base-content/80">
                    {t("settings.account.mfa.regenerateHint", {
                      defaultValue:
                        "Generating new recovery codes invalidates every existing one.",
                    })}
                  </p>
                </div>

                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("settings.account.mfa.currentPassword", {
                      defaultValue: "Current password",
                    })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="password"
                      placeholder="••••••••"
                      className="grow"
                      value={regeneratePassword}
                      onChange={(event) =>
                        setRegeneratePassword(event.target.value)
                      }
                      minLength={6}
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={t(
                        "settings.account.mfa.currentPasswordHintRegenerate",
                        {
                          defaultValue:
                            "Confirm your identity before generating new codes.",
                        },
                      )}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <button
                  type="submit"
                  className={`btn btn-primary w-full ${regenerateRecoveryCodes.isPending ? "loading" : ""}`}
                  disabled={isBusy}
                >
                  {t("settings.account.mfa.confirmRegenerate", {
                    defaultValue: "Generate new recovery codes",
                  })}
                </button>
              </form>
            )}

            {/* ── Recovery codes display ─────────────────────────── */}
            {revealedRecoveryCodes.length > 0 && (
              <div className="space-y-5">
                <div className="rounded-2xl border border-success/30 bg-success/10 p-4">
                  <div className="flex items-start gap-3">
                    <CheckCircle2 className="mt-0.5 h-5 w-5 text-success" />
                    <div>
                      <p className="text-sm font-semibold text-success">
                        {t("settings.account.mfa.recoveryCodesTitle", {
                          defaultValue: "Recovery codes",
                        })}
                      </p>
                      <p className="mt-1 text-sm text-base-content/80">
                        {t("settings.account.mfa.recoveryCodesHint", {
                          defaultValue:
                            "Store these one-time codes somewhere safe. Each code can be used once if your authenticator app is unavailable.",
                        })}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="rounded-2xl border border-base-300 bg-base-200/50 p-4">
                  <div className="grid gap-2 font-mono text-sm text-base-content sm:grid-cols-2">
                    {revealedRecoveryCodes.map((code) => (
                      <div
                        key={code}
                        className="rounded-lg border border-base-300 bg-base-100 px-3 py-2 text-center tracking-wider"
                      >
                        {code}
                      </div>
                    ))}
                  </div>
                </div>

                <button
                  type="button"
                  className="btn btn-outline btn-sm w-full gap-1.5"
                  onClick={() => copyToClipboard(recoveryCodesText)}
                >
                  <ClipboardCopy className="h-4 w-4" />
                  {t("common.copy", { defaultValue: "Copy all codes" })}
                </button>
              </div>
            )}

            {/* ── Bottom navigation ──────────────────────────────── */}
            <div className="divider my-0 text-xs text-base-content/70">
              {t("common.or", { defaultValue: "OR" })}
            </div>

            <div className="text-center">
              <p className="text-xs text-base-content/70">
                {t("auth.mfa.doneHere", {
                  defaultValue: "Done with MFA settings?",
                })}
              </p>
              <Link to={backTo} className="btn btn-link btn-sm mt-1 gap-1.5">
                <ArrowLeft className="h-3.5 w-3.5" />
                {isBootstrapOnboarding
                  ? t("auth.mfa.continueToApp", {
                      defaultValue: "Continue to Lumilio",
                    })
                  : t("auth.mfa.backToSettings", {
                      defaultValue: "Back to settings",
                    })}
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
