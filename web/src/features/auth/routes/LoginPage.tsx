import React, { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  AlertCircle,
  ArrowLeft,
  Info,
  KeyRound,
  LogIn,
  ShieldCheck,
  User,
} from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "../hooks/useAuth.ts";
import { useBootstrapStatus } from "../hooks/useBootstrapStatus.ts";
import type {
  ApiResult,
  AuthResponse,
  MFAMethod,
  PasskeyOptionsResponse,
  User as UserType,
} from "../auth.type.ts";
import {
  getPasskeyCredential,
  getPasskeySupport,
} from "../lib/webauthn.ts";
import {
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "../lib/credentialPolicy.ts";

type AuthRedirectState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

type LoginChallenge = {
  user: UserType | null;
  mfaToken: string;
  mfaMethods: MFAMethod[];
};

const RECOVERY_CODE_PATTERN = "[A-Za-z0-9]{4}-?[A-Za-z0-9]{4}";

function getApiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const apiError = error as { message?: string; error?: string };
    if (apiError.message) return apiError.message;
    if (apiError.error) return apiError.error;
  }
  return fallback;
}

const LoginPage: React.FC = () => {
  const { t } = useI18n();
  const bootstrapQuery = useBootstrapStatus();
  const {
    login,
    verifyMFA,
    completeAuth,
    dispatch,
    isAuthenticated,
    isLoading,
    error,
  } = useAuth();
  const passkeyOptionsMutation = $api.useMutation(
    "post",
    "/api/v1/auth/passkeys/login/options",
  );
  const passkeyVerifyMutation = $api.useMutation(
    "post",
    "/api/v1/auth/passkeys/login/verify",
  );
  const location = useLocation();
  const navigate = useNavigate();

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [showPasswordStep, setShowPasswordStep] = useState(false);
  const [challenge, setChallenge] = useState<LoginChallenge | null>(null);
  const [mfaCode, setMfaCode] = useState("");
  const [mfaMethod, setMfaMethod] = useState<MFAMethod>("totp");
  const [passkeyError, setPasskeyError] = useState<string | null>(null);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const bootstrapStatus = bootstrapQuery.data?.data;
  const isBootstrapMode = bootstrapStatus?.is_bootstrap_mode ?? false;
  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const displayName =
    challenge?.user?.display_name || challenge?.user?.username || username;
  const recoveryCodeAvailable =
    challenge?.mfaMethods.includes("recovery_code") ?? false;
  const passkeyBusy =
    passkeyOptionsMutation.isPending || passkeyVerifyMutation.isPending;
  const displayError = passkeyError ?? error;
  const passkeySupportReason = passkeySupport.reasonKey
    ? t(passkeySupport.reasonKey)
    : null;

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, redirectTo]);

  const handlePasswordLogin = async (
    event?: React.FormEvent<HTMLFormElement>,
  ) => {
    event?.preventDefault();
    setPasskeyError(null);
    try {
      const result = await login(username, password);
      if (result.status === "mfa_required") {
        setChallenge(result.challenge);
        setMfaMethod(
          result.challenge.mfaMethods.includes("totp")
            ? "totp"
            : (result.challenge.mfaMethods[0] ?? "totp"),
        );
        setMfaCode("");
        return;
      }
      navigate(redirectTo, { replace: true });
    } catch {
      // Auth context owns password errors.
    }
  };

  const handlePasskeyLogin = async (
    event?: React.FormEvent<HTMLFormElement>,
  ) => {
    event?.preventDefault();
    setPasskeyError(null);

    try {
      const optionsResponse = await passkeyOptionsMutation.mutateAsync({
        body: { username },
      });
      const optionsData =
        optionsResponse as ApiResult<PasskeyOptionsResponse> | undefined;
      if (!optionsData?.data) {
        throw new Error(
          optionsData?.message ||
            t("auth.login.passkeyStartError"),
        );
      }

      const credential = await getPasskeyCredential(optionsData.data.options);
      const verifyResponse = await passkeyVerifyMutation.mutateAsync({
        body: {
          challenge_token: optionsData.data.challenge_token,
          credential,
        },
      });
      const verifyData =
        verifyResponse as ApiResult<AuthResponse> | undefined;
      if (!verifyData?.data) {
        throw new Error(verifyData?.message || t("auth.login.passkeyVerifyError"));
      }

      await completeAuth(verifyData.data);
      navigate(redirectTo, { replace: true });
    } catch (passkeyAuthError) {
      setPasskeyError(
        getApiMessage(
          passkeyAuthError,
          t("auth.login.passkeyUnavailable"),
        ),
      );
    }
  };

  const handleVerifyMFA = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!challenge) return;

    try {
      await verifyMFA(challenge.mfaToken, mfaCode, mfaMethod);
      navigate(redirectTo, { replace: true });
    } catch {
      // Auth context owns MFA errors.
    }
  };

  const handleBack = () => {
    if (challenge) {
      setChallenge(null);
      setMfaCode("");
      setMfaMethod("totp");
      dispatch({ type: "AUTH_IDLE" });
      return;
    }

    setShowPasswordStep(false);
    setPassword("");
    setPasskeyError(null);
    dispatch({ type: "AUTH_IDLE" });
  };

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-base-200 px-4 py-12">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -right-48 -top-48 h-96 w-96 rounded-full bg-primary/8 blur-3xl" />
        <div className="absolute -bottom-48 -left-48 h-96 w-96 rounded-full bg-secondary/8 blur-3xl" />
      </div>

      <div className="relative w-full max-w-md">
        <div className="card bg-base-100 shadow-2xl ring-1 ring-base-content/5">
          <div className="card-body gap-7 p-8 sm:p-10">
            <div className="flex flex-col items-center gap-5 text-center">
              <div className="inline-flex items-center gap-4 text-3xl font-bold tracking-tight text-base-content sm:text-4xl">
                <img
                  src="/logo.png"
                  alt={t("auth.common.logoAlt", {
                    appName: t("app.name"),
                  })}
                  className="size-10 bg-contain object-contain sm:size-12"
                />
                <span>{t("app.name")}</span>
              </div>

              <div className="space-y-1.5">
                <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
                  {challenge
                    ? t("auth.login.verifyTitle", {
                        defaultValue: "Two-factor verification",
                      })
                    : showPasswordStep
                      ? t("auth.login.passwordTitle", {
                          defaultValue: "Enter your password",
                        })
                      : isBootstrapMode
                        ? t("auth.login.bootstrapTitle", {
                            defaultValue: "Set up your Admin account",
                          })
                        : t("auth.login.usernameTitle", {
                            defaultValue: "Continue with your username",
                          })}
                </h1>

                <p className="text-sm text-base-content/80">
                  {challenge
                    ? t("auth.login.verifySubtitle", {
                        defaultValue:
                          "Complete sign in with your authenticator app or a recovery code.",
                      })
                    : showPasswordStep
                      ? t("auth.login.passwordSubtitle", {
                          defaultValue:
                            "Use your password if you prefer not to use a passkey on this device.",
                        })
                      : isBootstrapMode
                        ? t("auth.login.bootstrapSubtitle", {
                            defaultValue:
                              "No users exist yet. Create the first account to claim Admin access.",
                          })
                        : t("auth.login.usernameSubtitle", {
                            defaultValue:
                              "Passkey is the primary sign-in path. Password remains available as fallback.",
                          })}
                </p>
              </div>
            </div>

            {displayError && (
              <div className="alert alert-error py-3 text-sm">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{t(displayError, { defaultValue: displayError })}</span>
              </div>
            )}

            {!challenge && isBootstrapMode ? (
              <div className="rounded-xl border border-warning/40 bg-warning/10 p-4">
                <p className="text-sm font-semibold text-warning">
                  {t("auth.login.bootstrapPromptTitle", {
                    defaultValue: "First-time setup required",
                  })}
                </p>
                <p className="mt-1.5 text-xs text-base-content/80">
                  {t("auth.login.bootstrapPromptBody", {
                    defaultValue:
                      "The first registration becomes Admin and will guide you through passkey or authenticator setup.",
                  })}
                </p>
                <Link
                  to="/register"
                  state={location.state}
                  className="btn btn-warning btn-sm mt-4 w-full"
                >
                  {t("auth.login.createAdmin", {
                    defaultValue: "Create the first Admin account",
                  })}
                </Link>
              </div>
            ) : challenge ? (
              <form className="flex flex-col gap-5" onSubmit={handleVerifyMFA}>
                <div className="flex items-start gap-3 rounded-xl border border-base-300 bg-base-200/70 p-4">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-primary/15">
                    <User className="h-4 w-4 text-primary" />
                  </div>
                  <div className="min-w-0">
                    <p className="text-sm font-semibold leading-snug">
                      {t("auth.login.verifyPrompt", {
                        defaultValue: "Continuing as {{name}}",
                        name: displayName,
                      })}
                    </p>
                    <p className="mt-0.5 text-xs text-base-content/80">
                      {mfaMethod === "recovery_code"
                        ? t("auth.login.recoveryHint", {
                            defaultValue:
                              "Enter one of your one-time recovery codes.",
                          })
                        : t("auth.login.totpHint", {
                            defaultValue:
                              "Enter the 6-digit code from your authenticator app.",
                          })}
                    </p>
                  </div>
                </div>

                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {mfaMethod === "recovery_code"
                      ? t("auth.login.recoveryCode", {
                          defaultValue: "Recovery code",
                        })
                      : t("auth.login.authenticatorCode", {
                          defaultValue: "Authenticator code",
                        })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <ShieldCheck className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="text"
                      inputMode={
                        mfaMethod === "recovery_code" ? "text" : "numeric"
                      }
                      autoComplete="one-time-code"
                      autoFocus
                      className="grow"
                      placeholder={
                        mfaMethod === "recovery_code"
                          ? t("auth.login.recoveryCodePlaceholder")
                          : t("auth.login.authenticatorCodePlaceholder")
                      }
                      value={mfaCode}
                      onChange={(event) => setMfaCode(event.target.value)}
                      pattern={
                        mfaMethod === "recovery_code"
                          ? RECOVERY_CODE_PATTERN
                          : "[0-9]{6}"
                      }
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={
                        mfaMethod === "recovery_code"
                          ? t("auth.login.recoveryValidation", {
                              defaultValue:
                                "Enter an 8-character recovery code.",
                            })
                          : t("auth.login.totpValidation", {
                              defaultValue:
                                "Enter a 6-digit authenticator code.",
                            })
                      }
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <div className="flex gap-2.5">
                  <button
                    type="button"
                    className="btn btn-ghost btn-square"
                    onClick={handleBack}
                    title={t("auth.login.back", { defaultValue: "Back" })}
                  >
                    <ArrowLeft className="h-4 w-4" />
                  </button>
                  <button
                    type="submit"
                    className={`btn btn-primary flex-1 ${isLoading ? "loading" : ""}`}
                    disabled={isLoading}
                  >
                    {!isLoading && <ShieldCheck className="h-4 w-4" />}
                    {isLoading
                      ? t("auth.login.verifying", {
                          defaultValue: "Verifying…",
                        })
                      : t("auth.login.verifyButton", {
                          defaultValue: "Verify and continue",
                        })}
                  </button>
                </div>

                {recoveryCodeAvailable && (
                  <div className="text-center">
                    <button
                      type="button"
                      className="btn btn-link btn-sm px-0 text-xs"
                      onClick={() =>
                        setMfaMethod((current) =>
                          current === "totp" ? "recovery_code" : "totp",
                        )
                      }
                    >
                      {mfaMethod === "totp"
                        ? t("auth.login.useRecoveryCode", {
                            defaultValue: "Use a recovery code instead",
                          })
                        : t("auth.login.useAuthenticatorCode", {
                            defaultValue: "Use an authenticator code instead",
                          })}
                    </button>
                  </div>
                )}
              </form>
            ) : (
              <form
                className="flex flex-col gap-4"
                onSubmit={
                  showPasswordStep || !passkeySupport.supported
                    ? handlePasswordLogin
                    : handlePasskeyLogin
                }
              >
                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("auth.login.username", { defaultValue: "Username" })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <User className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="text"
                      placeholder={t("auth.login.usernamePlaceholder")}
                      className="grow"
                      value={username}
                      onChange={(event) =>
                        setUsername(normalizeUsernameInput(event.target.value))
                      }
                      pattern={USERNAME_PATTERN}
                      minLength={USERNAME_MIN_LENGTH}
                      maxLength={USERNAME_MAX_LENGTH}
                      autoComplete="username webauthn"
                      autoFocus
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={t("auth.login.usernameHint", {
                        defaultValue: USERNAME_HINT,
                      })}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                {showPasswordStep && (
                  <fieldset className="fieldset">
                    <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                      {t("auth.login.password", { defaultValue: "Password" })}
                    </legend>
                    <label className="input input-bordered validator flex w-full items-center gap-2">
                      <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                      <input
                        type="password"
                        placeholder="••••••••"
                        className="grow"
                        value={password}
                        onChange={(event) => setPassword(event.target.value)}
                        autoComplete="current-password"
                        required
                      />
                    </label>
                  </fieldset>
                )}

                {!showPasswordStep && passkeySupportReason && (
                  <div className="rounded-xl border border-base-300 bg-base-200/60 p-4 text-sm text-base-content/80">
                    {passkeySupportReason}
                  </div>
                )}

                <div className="flex gap-2.5">
                  {showPasswordStep && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-square"
                      onClick={handleBack}
                      title={t("auth.login.back", { defaultValue: "Back" })}
                    >
                      <ArrowLeft className="h-4 w-4" />
                    </button>
                  )}

                  {showPasswordStep || !passkeySupport.supported ? (
                    <button
                      type="submit"
                      className={`btn btn-primary flex-1 ${isLoading ? "loading" : ""}`}
                      disabled={isLoading}
                    >
                      {!isLoading && <LogIn className="h-4 w-4" />}
                      {isLoading
                        ? t("auth.login.loading", {
                            defaultValue: "Signing in…",
                          })
                        : t("auth.login.submit", {
                            defaultValue: "Continue with password",
                          })}
                    </button>
                  ) : (
                    <>
                      <button
                        type="submit"
                        className={`btn btn-primary flex-1 ${passkeyBusy ? "loading" : ""}`}
                        disabled={passkeyBusy}
                      >
                        {!passkeyBusy && <ShieldCheck className="h-4 w-4" />}
                        {passkeyBusy
                          ? t("auth.login.passkeyLoading", {
                              defaultValue: "Opening passkey…",
                            })
                          : t("auth.login.passkeySubmit", {
                              defaultValue: "Continue with Passkey",
                            })}
                      </button>
                      <button
                        type="button"
                        className="btn btn-outline flex-1"
                        onClick={() => {
                          setShowPasswordStep(true);
                          setPasskeyError(null);
                        }}
                      >
                        {t("auth.login.usePasswordInstead", {
                          defaultValue: "Use password instead",
                        })}
                      </button>
                    </>
                  )}
                </div>
              </form>
            )}

            <div className="divider my-0 text-xs text-base-content/70">
              {t("common.or", { defaultValue: "OR" })}
            </div>

            <div className="text-center">
              <p className="text-xs text-base-content/70">
                {t("auth.login.needAccount", {
                  defaultValue: "Need an account?",
                })}
              </p>
              <Link
                to="/register"
                state={location.state}
                className="btn btn-link btn-sm mt-1"
              >
                {t("auth.login.register", {
                  defaultValue: "Create an account",
                })}
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
