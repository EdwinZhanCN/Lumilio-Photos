import React from "react";
import { Link, useLocation } from "react-router-dom";
import {
  AlertCircle,
  CheckCircle2,
  Info,
  KeyRound,
  ShieldCheck,
  Smartphone,
  User,
  UserPlus,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useRegistrationFlow } from "../hooks/useRegistrationFlow.ts";
import {
  PASSWORD_HINT,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  PASSWORD_PATTERN,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "../lib/credentialPolicy.ts";

type RegistrationFormProps = {
  credentialTitle: string;
  credentialSubtitle: string;
  credentialSubmitLabel: string;
  credentialPrompt?: {
    title: string;
    body: string;
  };
  showLoginLink?: boolean;
};

const RegistrationForm: React.FC<RegistrationFormProps> = ({
  credentialTitle,
  credentialSubtitle,
  credentialSubmitLabel,
  credentialPrompt,
  showLoginLink = true,
}) => {
  const { t } = useI18n();
  const location = useLocation();
  const {
    step,
    username,
    setUsername,
    password,
    setPassword,
    confirmPassword,
    setConfirmPassword,
    confirmPasswordRef,
    confirmPasswordMessage,
    capabilityMessage,
    totpSetup,
    totpCode,
    setTotpCode,
    recoveryCodes,
    qrCodeDataURL,
    displayError,
    isBusy,
    handleStartRegistration,
    handleCreatePasskey,
    handleUseAuthenticatorApp,
    handleCompleteTotp,
    handleFinish,
  } = useRegistrationFlow();

  const title =
    step === "credentials"
      ? credentialTitle
      : step === "choose"
        ? t("auth.register.securityChoiceTitle", {
            defaultValue: "Choose your sign-in method",
          })
        : step === "totp"
          ? t("auth.register.totpTitle", {
              defaultValue: "Set up Authenticator App",
            })
          : t("auth.register.recoveryTitle", {
              defaultValue: "Save your recovery codes",
            });

  const subtitle =
    step === "credentials"
      ? credentialSubtitle
      : step === "choose"
        ? t("auth.register.securityChoiceSubtitle", {
            defaultValue:
              "Passkey is recommended when the current device and origin support it.",
          })
        : step === "totp"
          ? t("auth.register.totpSubtitle", {
              defaultValue:
                "Scan the QR code with Duo, 1Password, or any authenticator app, then verify once.",
            })
          : t("auth.register.recoverySubtitle", {
              defaultValue:
                "These one-time codes are the only fallback if you lose access to your authenticator.",
            });

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-base-200 px-4 py-12">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -right-48 -top-48 h-96 w-96 rounded-full bg-primary/8 blur-3xl" />
        <div className="absolute -bottom-48 -left-48 h-96 w-96 rounded-full bg-secondary/8 blur-3xl" />
      </div>

      <div className="relative w-full max-w-lg">
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
                  {title}
                </h1>

                <p className="text-sm text-base-content/80">{subtitle}</p>
              </div>
            </div>

            {displayError && (
              <div className="alert alert-error py-3 text-sm">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{t(displayError, { defaultValue: displayError })}</span>
              </div>
            )}

            {step === "credentials" && credentialPrompt && (
              <div className="rounded-xl border border-primary/40 bg-primary/10 p-4">
                <p className="text-sm font-semibold text-primary">
                  {credentialPrompt.title}
                </p>
                <p className="mt-1.5 text-xs text-base-content/80">
                  {credentialPrompt.body}
                </p>
              </div>
            )}

            {step === "credentials" && (
              <form
                className="flex flex-col gap-2.5"
                onSubmit={handleStartRegistration}
              >
                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("auth.register.username", { defaultValue: "Username" })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <User className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="text"
                      placeholder={t("auth.register.usernamePlaceholder")}
                      className="grow"
                      value={username}
                      onChange={(event) =>
                        setUsername(normalizeUsernameInput(event.target.value))
                      }
                      pattern={USERNAME_PATTERN}
                      minLength={USERNAME_MIN_LENGTH}
                      maxLength={USERNAME_MAX_LENGTH}
                      autoComplete="username"
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={t("auth.register.usernameHint", {
                        defaultValue: USERNAME_HINT,
                      })}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("auth.register.password", { defaultValue: "Password" })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      type="password"
                      placeholder="••••••••"
                      className="grow"
                      value={password}
                      onChange={(event) => setPassword(event.target.value)}
                      minLength={PASSWORD_MIN_LENGTH}
                      maxLength={PASSWORD_MAX_LENGTH}
                      pattern={PASSWORD_PATTERN}
                      autoComplete="new-password"
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={t("auth.register.passwordHint", {
                        defaultValue: PASSWORD_HINT,
                      })}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <fieldset className="fieldset">
                  <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                    {t("auth.register.confirmPassword", {
                      defaultValue: "Confirm password",
                    })}
                  </legend>
                  <label className="input input-bordered validator flex w-full items-center gap-2">
                    <ShieldCheck className="h-4 w-4 shrink-0 text-base-content/70" />
                    <input
                      ref={confirmPasswordRef}
                      type="password"
                      placeholder="••••••••"
                      className="grow"
                      value={confirmPassword}
                      onChange={(event) => setConfirmPassword(event.target.value)}
                      autoComplete="new-password"
                      required
                    />
                    <div
                      className="tooltip tooltip-left cursor-help"
                      data-tip={confirmPasswordMessage}
                    >
                      <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                    </div>
                  </label>
                </fieldset>

                <button
                  type="submit"
                  className={`btn btn-primary mt-2 w-full ${isBusy ? "loading" : ""}`}
                  disabled={isBusy}
                >
                  {!isBusy && <UserPlus className="h-4 w-4" />}
                  {isBusy
                    ? t("auth.register.loading", {
                        defaultValue: "Preparing account...",
                      })
                    : credentialSubmitLabel}
                </button>
              </form>
            )}

            {step === "choose" && (
              <div className="space-y-4">
                {capabilityMessage && (
                  <div className="rounded-xl border border-base-300 bg-base-200/60 p-4 text-sm text-base-content/80">
                    {t(capabilityMessage, { defaultValue: capabilityMessage })}
                  </div>
                )}

                <div className="rounded-2xl border border-primary/30 bg-primary/5 p-5">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-lg font-semibold text-base-content">
                        {t("auth.register.passkeyCardTitle", {
                          defaultValue: "Create Passkey",
                        })}
                      </h2>
                      <p className="mt-1 text-sm text-base-content/80">
                        {t("auth.register.passkeyCardBody", {
                          defaultValue:
                            "Use the current device's native passkey flow for the fastest and strongest sign-in.",
                        })}
                      </p>
                    </div>
                    <ShieldCheck className="h-5 w-5 text-primary" />
                  </div>
                  <button
                    type="button"
                    className={`btn btn-primary mt-4 w-full ${isBusy ? "loading" : ""}`}
                    disabled={isBusy}
                    onClick={() => void handleCreatePasskey()}
                  >
                    {t("auth.register.passkeyAction", {
                      defaultValue: "Create Passkey",
                    })}
                  </button>
                </div>

                <div className="rounded-2xl border border-base-300 bg-base-200/50 p-5">
                  <div className="flex items-start justify-between gap-4">
                    <div>
                      <h2 className="text-lg font-semibold text-base-content">
                        {t("auth.register.totpCardTitle", {
                          defaultValue: "Use Authenticator App instead",
                        })}
                      </h2>
                      <p className="mt-1 text-sm text-base-content/80">
                        {t("auth.register.totpCardBody", {
                          defaultValue:
                            "Choose TOTP if you prefer Duo, 1Password, Google Authenticator, or another app.",
                        })}
                      </p>
                    </div>
                    <Smartphone className="h-5 w-5 text-base-content/70" />
                  </div>
                  <button
                    type="button"
                    className="btn btn-outline mt-4 w-full"
                    disabled={isBusy}
                    onClick={() => void handleUseAuthenticatorApp()}
                  >
                    {t("auth.register.totpAction", {
                      defaultValue: "Use Authenticator App",
                    })}
                  </button>
                </div>
              </div>
            )}

            {step === "totp" && totpSetup && (
              <form className="space-y-5" onSubmit={handleCompleteTotp}>
                <div className="grid gap-5 lg:grid-cols-[minmax(0,15rem)_minmax(0,1fr)]">
                  <div className="rounded-2xl border border-base-300 bg-base-200/50 p-4">
                    <div className="mx-auto aspect-square w-full max-w-60 rounded-2xl border border-base-300 bg-base-100 p-3 shadow-sm">
                      {qrCodeDataURL ? (
                        <img
                          src={qrCodeDataURL}
                          alt={t("auth.register.totpQrAlt")}
                          className="size-full rounded-xl bg-white object-contain"
                        />
                      ) : (
                        <div className="skeleton size-full rounded-xl" />
                      )}
                    </div>
                  </div>

                  <div className="space-y-4">
                    {capabilityMessage && (
                      <div className="rounded-xl border border-base-300 bg-base-200/60 p-4 text-sm text-base-content/80">
                        {t(capabilityMessage, { defaultValue: capabilityMessage })}
                      </div>
                    )}

                    <div className="rounded-xl border border-base-300 bg-base-200/50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-wider text-base-content/70">
                        {t("auth.register.manualKey", {
                          defaultValue: "Setup key",
                        })}
                      </p>
                      <p className="mt-2 break-all font-mono text-sm text-base-content">
                        {totpSetup.secret}
                      </p>
                    </div>

                    <fieldset className="fieldset">
                      <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                        {t("auth.register.verifyCode", {
                          defaultValue: "Verification code",
                        })}
                      </legend>
                      <label className="input input-bordered validator flex w-full items-center gap-2">
                        <ShieldCheck className="h-4 w-4 shrink-0 text-base-content/70" />
                        <input
                          type="text"
                          inputMode="numeric"
                          placeholder="123456"
                          className="grow"
                          value={totpCode}
                          onChange={(event) => setTotpCode(event.target.value)}
                          pattern="[0-9]{6}"
                          autoComplete="one-time-code"
                          required
                        />
                      </label>
                    </fieldset>
                  </div>
                </div>

                <button
                  type="submit"
                  className={`btn btn-primary w-full ${isBusy ? "loading" : ""}`}
                  disabled={isBusy}
                >
                  {!isBusy && <ShieldCheck className="h-4 w-4" />}
                  {t("auth.register.verifyAndFinish", {
                    defaultValue: "Verify and finish setup",
                  })}
                </button>
              </form>
            )}

            {step === "recovery" && (
              <div className="space-y-5">
                <div className="rounded-2xl border border-success/30 bg-success/10 p-4">
                  <div className="flex items-start gap-3">
                    <CheckCircle2 className="mt-0.5 h-5 w-5 text-success" />
                    <div>
                      <p className="text-sm font-semibold text-success">
                        {t("auth.register.recoveryReadyTitle", {
                          defaultValue: "Account secured successfully",
                        })}
                      </p>
                      <p className="mt-1 text-sm text-base-content/80">
                        {t("auth.register.recoveryReadyBody", {
                          defaultValue:
                            "Store these recovery codes somewhere safe before continuing.",
                        })}
                      </p>
                    </div>
                  </div>
                </div>

                <div className="rounded-2xl border border-base-300 bg-base-200/50 p-4">
                  <div className="grid gap-2 font-mono text-sm text-base-content">
                    {recoveryCodes.map((code) => (
                      <div
                        key={code}
                        className="rounded-lg border border-base-300 bg-base-100 px-3 py-2"
                      >
                        {code}
                      </div>
                    ))}
                  </div>
                </div>

                <button
                  type="button"
                  className="btn btn-primary w-full"
                  onClick={handleFinish}
                >
                  {t("auth.register.finish", {
                    defaultValue: "Continue to Lumilio",
                  })}
                </button>
              </div>
            )}

            {showLoginLink && (
              <>
                <div className="divider my-0 text-xs text-base-content/70">
                  {t("common.or", { defaultValue: "OR" })}
                </div>

                <div className="text-center">
                  <Link
                    to="/login"
                    state={location.state}
                    className="btn btn-link btn-sm"
                  >
                    {t("auth.register.login", {
                      defaultValue: "Go to login",
                    })}
                  </Link>
                </div>
              </>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default RegistrationForm;
