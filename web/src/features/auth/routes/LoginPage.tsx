import React, { useEffect, useMemo, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { ArrowLeft, Fingerprint, KeyRound, Smartphone, User } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "../hooks/useAuth.ts";
import type { MFAMethod, User as UserType } from "../auth.type.ts";
import { getPasskeyCredential, getPasskeySupport } from "../lib/webauthn.ts";
import {
  PASSWORD_HINT,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "../lib/credentialPolicy.ts";
import {
  AuthShell,
  Btn,
  CardHead,
  Field,
  InlineError,
  OtpInput,
  PasswordField,
  TextInput,
} from "../components/ui.tsx";

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

type LoginStep = "identify" | "passkey" | "password" | "mfa";

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
  const { login, verifyMFA, completeAuth, dispatch, isAuthenticated, isLoading, error } = useAuth();
  const loginOptionsMutation = $api.useMutation("post", "/api/v1/auth/login/options");
  const passkeyOptionsMutation = $api.useMutation("post", "/api/v1/auth/passkeys/login/options");
  const passkeyVerifyMutation = $api.useMutation("post", "/api/v1/auth/passkeys/login/verify");
  const location = useLocation();
  const navigate = useNavigate();

  const [step, setStep] = useState<LoginStep>("identify");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<LoginChallenge | null>(null);
  const [mfaCode, setMfaCode] = useState("");
  const [mfaMethod, setMfaMethod] = useState<MFAMethod>("totp");
  const [passkeyError, setPasskeyError] = useState<string | null>(null);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [passkeyUnsupportedNote, setPasskeyUnsupportedNote] = useState<string | null>(null);
  const passwordInputRef = React.useRef<HTMLInputElement>(null);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const displayName = challenge?.user?.display_name || challenge?.user?.username || username;
  const recoveryCodeAvailable = challenge?.mfaMethods.includes("recovery_code") ?? false;
  const passkeyBusy = passkeyOptionsMutation.isPending || passkeyVerifyMutation.isPending;
  const identifyBusy = loginOptionsMutation.isPending;
  const displayError = optionsError ?? passkeyError ?? error;
  const passkeySupportReason = passkeySupport.reasonKey ? t(passkeySupport.reasonKey) : null;
  const usernameValid = username.trim().length >= USERNAME_MIN_LENGTH;

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      void navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, redirectTo]);

  useEffect(() => {
    if (step !== "password") return;
    passwordInputRef.current?.focus();
  }, [step]);

  const clearStepErrors = () => {
    setPasskeyError(null);
    setOptionsError(null);
    dispatch({ type: "AUTH_IDLE" });
  };

  const goToIdentify = () => {
    setStep("identify");
    setPassword("");
    setPasskeyUnsupportedNote(null);
    setChallenge(null);
    setMfaCode("");
    setMfaMethod("totp");
    clearStepErrors();
  };

  const goToPassword = (note: string | null = null) => {
    setStep("password");
    setPassword("");
    setPasskeyUnsupportedNote(note);
    clearStepErrors();
  };

  const handleIdentify = async (event?: React.FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    clearStepErrors();
    setPasskeyUnsupportedNote(null);

    if (!usernameValid) {
      setOptionsError(
        t("auth.login.usernameRequired", {
          defaultValue: "Enter your username to continue.",
        }),
      );
      return;
    }

    try {
      const response = await loginOptionsMutation.mutateAsync({
        body: { username },
      });
      const options = response;
      if (!options) {
        throw new Error(
          t("auth.login.optionsError", {
            defaultValue: "Unable to continue with this username.",
          }),
        );
      }

      if (options.passkey && passkeySupport.supported) {
        setStep("passkey");
        return;
      }

      const note =
        options.passkey && !passkeySupport.supported ? passkeySupportReason : null;
      goToPassword(note);
    } catch (identifyError) {
      setOptionsError(
        getApiMessage(
          identifyError,
          t("auth.login.optionsError", {
            defaultValue: "Unable to continue with this username.",
          }),
        ),
      );
    }
  };

  const handlePasswordLogin = async (event?: React.FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    setPasskeyError(null);
    setOptionsError(null);
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
        setStep("mfa");
        return;
      }
      void navigate(redirectTo, { replace: true });
    } catch {
      // Auth context owns password errors.
    }
  };

  const handlePasskeyLogin = async () => {
    setPasskeyError(null);
    setOptionsError(null);

    try {
      const optionsResponse = await passkeyOptionsMutation.mutateAsync({
        body: { username },
      });
      const optionsData = optionsResponse;
      if (!optionsData?.challenge_token) {
        throw new Error(t("auth.login.passkeyStartError"));
      }

      const credential = await getPasskeyCredential(optionsData.options);
      const verifyResponse = await passkeyVerifyMutation.mutateAsync({
        body: {
          challenge_token: optionsData.challenge_token,
          credential,
        },
      });
      const verifyData = verifyResponse;
      if (!verifyData) {
        throw new Error(t("auth.login.passkeyVerifyError"));
      }

      await completeAuth(verifyData);
      void navigate(redirectTo, { replace: true });
    } catch (passkeyAuthError) {
      setPasskeyError(getApiMessage(passkeyAuthError, t("auth.login.passkeyUnavailable")));
    }
  };

  const handleVerifyMFA = async (code?: string) => {
    if (!challenge) return;
    const value = code ?? mfaCode;
    try {
      await verifyMFA(challenge.mfaToken, value, mfaMethod);
      void navigate(redirectTo, { replace: true });
    } catch {
      setMfaCode("");
      // Auth context owns MFA errors.
    }
  };

  const handleBackFromMFA = () => {
    setChallenge(null);
    setMfaCode("");
    setMfaMethod("totp");
    setStep("password");
    clearStepErrors();
  };

  /* ----------------------------------------------------- MFA verify view --- */
  if (step === "mfa" && challenge) {
    const isRecovery = mfaMethod === "recovery_code";
    return (
      <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
        <AuthShell appName={t("app.name", { defaultValue: "Lumilio Photos" })}>
          <CardHead
            icon={isRecovery ? KeyRound : Smartphone}
            title={
              isRecovery
                ? t("auth.login.recoveryTitle", {
                    defaultValue: "Enter a recovery code",
                  })
                : t("auth.login.verifyTitle", {
                    defaultValue: "Two-factor authentication",
                  })
            }
            sub={
              isRecovery
                ? t("auth.login.recoveryHint", {
                    defaultValue: "Use one of the codes you saved when setting up your account.",
                  })
                : t("auth.login.verifyPrompt", {
                    defaultValue:
                      "Enter the 6-digit code from your authenticator app for {{name}}.",
                    name: displayName,
                  })
            }
          />

          {displayError && (
            <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
          )}

          {isRecovery ? (
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                void handleVerifyMFA();
              }}
            >
              <Field
                label={t("auth.login.recoveryCode", {
                  defaultValue: "Recovery code",
                })}
              >
                <TextInput
                  icon={KeyRound}
                  placeholder="xxxxx-xxxxx"
                  value={mfaCode}
                  onChange={(e) => setMfaCode(e.target.value)}
                  autoComplete="one-time-code"
                  autoFocus
                />
              </Field>
              <Btn
                type="submit"
                variant="primary"
                loading={isLoading}
                disabled={mfaCode.trim().length < 8}
              >
                {t("auth.login.useRecoveryCode", {
                  defaultValue: "Use recovery code",
                })}
              </Btn>
            </form>
          ) : (
            <div className="flex flex-col gap-4">
              <OtpInput
                value={mfaCode}
                onChange={setMfaCode}
                onComplete={(value) => void handleVerifyMFA(value)}
              />
              <Btn
                variant="primary"
                loading={isLoading}
                disabled={mfaCode.length < 6}
                onClick={() => void handleVerifyMFA()}
              >
                {t("auth.login.verifyButton", {
                  defaultValue: "Verify",
                })}
              </Btn>
            </div>
          )}

          <div className="flex flex-col items-center gap-2 text-sm">
            {recoveryCodeAvailable && (
              <button
                type="button"
                onClick={() => {
                  setMfaMethod((m) => (m === "totp" ? "recovery_code" : "totp"));
                  setMfaCode("");
                }}
                className="font-medium text-base-content/55 hover:text-base-content"
              >
                {isRecovery
                  ? t("auth.login.useAuthenticatorCode", {
                      defaultValue: "Use an authenticator code instead",
                    })
                  : t("auth.login.useRecoveryCodeInstead", {
                      defaultValue: "Can’t access your app? Use a recovery code",
                    })}
              </button>
            )}
            <button
              type="button"
              onClick={handleBackFromMFA}
              className="flex items-center gap-1 text-base-content/40 hover:text-base-content/65"
            >
              <ArrowLeft size={14} />{" "}
              {t("auth.login.backToSignIn", {
                defaultValue: "Back to sign in",
              })}
            </button>
          </div>
        </AuthShell>
      </div>
    );
  }

  /* ------------------------------------------------------ identify step --- */
  if (step === "identify") {
    return (
      <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
        <AuthShell appName={t("app.name", { defaultValue: "Lumilio Photos" })}>
          <CardHead
            title={t("auth.login.title", { defaultValue: "Sign in to Lumilio" })}
            sub={t("auth.login.identifySubtitle", {
              defaultValue: "Enter your username to continue.",
            })}
          />

          {displayError && (
            <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
          )}

          <form className="flex flex-col gap-4" onSubmit={(e) => void handleIdentify(e)}>
            <Field
              label={t("auth.login.username", { defaultValue: "Username" })}
              hint={t("auth.login.usernameHint", { defaultValue: USERNAME_HINT })}
            >
              <TextInput
                icon={User}
                type="text"
                placeholder={t("auth.login.usernamePlaceholder", {
                  defaultValue: "your-username",
                })}
                value={username}
                onChange={(e) => setUsername(normalizeUsernameInput(e.target.value))}
                pattern={USERNAME_PATTERN}
                minLength={USERNAME_MIN_LENGTH}
                maxLength={USERNAME_MAX_LENGTH}
                autoComplete="username webauthn"
                autoFocus
              />
            </Field>
            <Btn type="submit" variant="primary" loading={identifyBusy} disabled={!usernameValid}>
              {t("auth.login.continue", {
                defaultValue: "Continue",
              })}
            </Btn>
          </form>

          <div className="text-center text-sm text-base-content/55">
            {t("auth.login.registerPrompt", { defaultValue: "New to Lumilio?" })}{" "}
            <Link
              to="/register"
              state={location.state}
              className="font-medium text-base-content underline-offset-2 hover:underline"
            >
              {t("auth.login.register", { defaultValue: "Create an account" })}
            </Link>
          </div>
        </AuthShell>
      </div>
    );
  }

  /* -------------------------------------------------------- passkey step --- */
  if (step === "passkey") {
    return (
      <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
        <AuthShell appName={t("app.name", { defaultValue: "Lumilio Photos" })}>
          <CardHead
            icon={Fingerprint}
            title={t("auth.login.title", { defaultValue: "Sign in to Lumilio" })}
            sub={t("auth.login.passkeySubtitle", {
              defaultValue: "Continue as {{username}} with a passkey.",
              username,
            })}
          />

          {displayError && (
            <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
          )}

          <Btn
            variant="primary"
            icon={Fingerprint}
            loading={passkeyBusy}
            onClick={() => void handlePasskeyLogin()}
          >
            {t("auth.login.passkeySubmit", {
              defaultValue: "Sign in with a passkey",
            })}
          </Btn>

          <div className="flex flex-col items-center gap-2 text-sm">
            <button
              type="button"
              onClick={() => goToPassword(null)}
              className="font-medium text-base-content/55 hover:text-base-content"
            >
              {t("auth.login.usePasswordInstead", {
                defaultValue: "Use password instead",
              })}
            </button>
            <button
              type="button"
              onClick={goToIdentify}
              className="flex items-center gap-1 text-base-content/40 hover:text-base-content/65"
            >
              <ArrowLeft size={14} />{" "}
              {t("auth.login.backToUsername", {
                defaultValue: "Use a different username",
              })}
            </button>
          </div>
        </AuthShell>
      </div>
    );
  }

  /* ------------------------------------------------------- password step --- */
  return (
    <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
      <AuthShell appName={t("app.name", { defaultValue: "Lumilio Photos" })}>
        <CardHead
          title={t("auth.login.title", { defaultValue: "Sign in to Lumilio" })}
          sub={t("auth.login.passwordSubtitle", {
            defaultValue: "Enter the password for {{username}}.",
            username,
          })}
        />

        {passkeyUnsupportedNote && (
          <div className="rounded-xl border border-base-200 bg-base-200/50 px-4 py-3 text-sm text-base-content/65">
            {passkeyUnsupportedNote}
          </div>
        )}

        {displayError && (
          <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
        )}

        <form className="flex flex-col gap-4" onSubmit={(e) => void handlePasswordLogin(e)}>
          <PasswordField
            label={t("auth.login.password", { defaultValue: "Password" })}
            hint={t("auth.register.passwordHint", {
              defaultValue: PASSWORD_HINT,
            })}
            value={password}
            onChange={setPassword}
            autoComplete="current-password"
            inputRef={passwordInputRef}
          />
          <Btn type="submit" variant="primary" loading={isLoading} disabled={password.length === 0}>
            {t("auth.login.signIn", {
              defaultValue: "Sign in",
            })}
          </Btn>
        </form>

        <div className="flex flex-col items-center gap-2 text-sm">
          <button
            type="button"
            onClick={goToIdentify}
            className="flex items-center gap-1 text-base-content/40 hover:text-base-content/65"
          >
            <ArrowLeft size={14} />{" "}
            {t("auth.login.backToUsername", {
              defaultValue: "Use a different username",
            })}
          </button>
        </div>
      </AuthShell>
    </div>
  );
};

export default LoginPage;
