import React from "react";
import { Link } from "react-router-dom";
import { ArrowLeft, Fingerprint, KeyRound, Smartphone, User } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  PASSWORD_HINT,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "../../model/credentialPolicy.ts";
import {
  AuthShell,
  Btn,
  CardHead,
  Field,
  InlineError,
  OtpInput,
  PasswordField,
  TextInput,
} from "../../components/ui";
import { useLoginFlow } from "./useLoginFlow.ts";

const LoginFlow: React.FC = () => {
  const { t } = useI18n();
  const {
    step,
    challenge,
    username,
    setUsername,
    password,
    setPassword,
    mfaCode,
    setMfaCode,
    mfaMethod,
    displayName,
    recoveryCodeAvailable,
    passkeyBusy,
    identifyBusy,
    displayError,
    usernameValid,
    isLoading,
    passkeyUnsupportedNote,
    passwordInputRef,
    registrationState,
    goToIdentify,
    goToPassword,
    handleIdentify,
    handlePasswordLogin,
    handlePasskeyLogin,
    handleVerifyMFA,
    handleBackFromMFA,
    toggleMFAMethod,
  } = useLoginFlow();

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
                  toggleMFAMethod();
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
              state={registrationState}
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

export default LoginFlow;
