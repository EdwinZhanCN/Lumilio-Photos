import React, { type FormEvent } from "react";
import { Link, useLocation } from "react-router-dom";
import { Fingerprint, Info, KeyRound, Smartphone, User } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useRegistrationFlow } from "../hooks/useRegistrationFlow.ts";
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
  FlowSteps,
  InlineError,
  PasskeyAffordance,
  PasswordField,
  RecoveryCodesPanel,
  TextInput,
  TotpSetupPanel,
} from "./ui.tsx";

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
    passkeySupported,
    totpSetup,
    totpCode,
    setTotpCode,
    recoveryCodes,
    displayError,
    isBusy,
    handleStartRegistration,
    handleCreatePasskey,
    handleSkipPasskey,
    handleCompleteTotp,
    handleSkipTotp,
    handleFinish,
  } = useRegistrationFlow();

  const appName = t("app.name", { defaultValue: "Lumilio Photos" });
  const stepAccount = t("auth.register.stepAccount", { defaultValue: "Account" });
  const stepAuthenticator = t("auth.register.stepAuthenticator", {
    defaultValue: "Authenticator",
  });
  const stepPasskey = t("auth.register.stepPasskey", { defaultValue: "Passkey" });
  const stepRecovery = t("auth.register.stepRecovery", { defaultValue: "Recovery" });
  const steps = passkeySupported
    ? [stepAccount, stepAuthenticator, stepPasskey, stepRecovery]
    : [stepAccount, stepAuthenticator, stepRecovery];
  const stepIndex: Record<string, number> = passkeySupported
    ? { credentials: 0, totp: 1, passkey: 2, recovery: 3 }
    : { credentials: 0, totp: 1, recovery: 2 };

  const submitTotp = () => {
    void handleCompleteTotp({
      preventDefault: () => undefined,
    } as FormEvent<HTMLFormElement>);
  };

  return (
    <div className="grid min-h-screen place-items-center bg-base-200 px-4 py-10">
      <AuthShell width={460} appName={appName}>
        {step !== "credentials" && <FlowSteps steps={steps} current={stepIndex[step] ?? 0} />}

        {displayError && (
          <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
        )}

        {step === "credentials" && (
          <>
            <CardHead title={credentialTitle} sub={credentialSubtitle} />
            <form className="flex flex-col gap-4" onSubmit={handleStartRegistration}>
              <Field
                label={t("auth.register.username", { defaultValue: "Username" })}
                hint={t("auth.register.usernameHint", {
                  defaultValue: USERNAME_HINT,
                })}
              >
                <TextInput
                  icon={User}
                  type="text"
                  placeholder={t("auth.register.usernamePlaceholder", {
                    defaultValue: "your-username",
                  })}
                  value={username}
                  onChange={(e) => setUsername(normalizeUsernameInput(e.target.value))}
                  pattern={USERNAME_PATTERN}
                  minLength={USERNAME_MIN_LENGTH}
                  maxLength={USERNAME_MAX_LENGTH}
                  autoComplete="username"
                  required
                />
              </Field>

              <PasswordField
                label={t("auth.register.password", { defaultValue: "Password" })}
                hint={t("auth.register.passwordHint", {
                  defaultValue: PASSWORD_HINT,
                })}
                value={password}
                onChange={setPassword}
                meter
                autoComplete="new-password"
              />

              <PasswordField
                label={t("auth.register.confirmPassword", {
                  defaultValue: "Confirm password",
                })}
                hint={t("auth.register.confirmPasswordHint", {
                  defaultValue: "Passwords must match exactly.",
                })}
                value={confirmPassword}
                onChange={setConfirmPassword}
                placeholder={t("auth.register.confirmPasswordPlaceholder", {
                  defaultValue: "Re-enter password",
                })}
                autoComplete="new-password"
                inputRef={confirmPasswordRef}
              />

              {credentialPrompt ? (
                <div className="rounded-xl border border-primary/30 bg-primary/5 px-4 py-3">
                  <p className="text-sm font-semibold text-primary">{credentialPrompt.title}</p>
                  <p className="mt-1 text-xs text-base-content/70">{credentialPrompt.body}</p>
                </div>
              ) : (
                <div className="flex items-start gap-2 rounded-xl bg-base-200/50 px-3.5 py-2.5 text-xs text-base-content/55">
                  <Info size={14} className="mt-0.5 shrink-0 text-base-content/40" />
                  {t("auth.register.mfaOptionalNotice", {
                    defaultValue:
                      "You can add two-factor authentication next — it’s optional, and you can do it later in settings.",
                  })}
                </div>
              )}

              <Btn type="submit" variant="primary" loading={isBusy}>
                {credentialSubmitLabel}
              </Btn>
            </form>

            {showLoginLink && (
              <div className="text-center text-sm text-base-content/55">
                {t("auth.register.haveAccount", {
                  defaultValue: "Already have an account?",
                })}{" "}
                <Link
                  to="/login"
                  state={location.state}
                  className="font-medium text-base-content underline-offset-2 hover:underline"
                >
                  {t("auth.register.login", { defaultValue: "Sign in" })}
                </Link>
              </div>
            )}
          </>
        )}

        {step === "totp" && totpSetup && (
          <>
            <CardHead
              icon={Smartphone}
              tone="primary"
              title={t("auth.register.totpTitle", {
                defaultValue: "Add your authenticator",
              })}
              sub={t("auth.register.totpSubtitle", {
                defaultValue: "Optional — scan the code with your authenticator app.",
              })}
            />
            <TotpSetupPanel
              otpauthUri={totpSetup.otpauth_uri ?? ""}
              secret={totpSetup.secret ?? ""}
              code={totpCode}
              onCodeChange={setTotpCode}
              onVerify={submitTotp}
              invalid={Boolean(displayError)}
              busy={isBusy}
              verifyLabel={t("auth.register.verifyAndEnable", {
                defaultValue: "Verify & enable",
              })}
            />
            <button
              type="button"
              onClick={handleSkipTotp}
              disabled={isBusy}
              className="text-center text-sm font-medium text-base-content/45 hover:text-base-content/70"
            >
              {t("auth.register.skipTotp", {
                defaultValue: "Skip for now",
              })}
            </button>
          </>
        )}

        {step === "passkey" && (
          <>
            <CardHead
              icon={Fingerprint}
              tone="primary"
              title={t("auth.register.passkeyStepTitle", {
                defaultValue: "Add a passkey",
              })}
              sub={t("auth.register.passkeyStepSubtitle", {
                defaultValue: "Optional — the fastest, safest way to sign in next time.",
              })}
            />
            <PasskeyAffordance
              headline={t("auth.register.passkeyHeadline", {
                defaultValue: "Sign in with your face or fingerprint",
              })}
              description={t("auth.register.passkeyDescription", {
                defaultValue:
                  "No password to remember. Your passkey stays on this device and can’t be phished.",
              })}
            />
            <Btn
              variant="primary"
              icon={Fingerprint}
              loading={isBusy}
              onClick={() => void handleCreatePasskey()}
            >
              {t("auth.register.passkeyAction", {
                defaultValue: "Create a passkey",
              })}
            </Btn>
            <button
              type="button"
              onClick={handleSkipPasskey}
              disabled={isBusy}
              className="text-center text-sm font-medium text-base-content/45 hover:text-base-content/70"
            >
              {t("auth.register.skipPasskey", {
                defaultValue: "Skip for now",
              })}
            </button>
          </>
        )}

        {step === "recovery" && (
          <>
            <CardHead
              icon={KeyRound}
              tone="warning"
              title={t("auth.register.recoveryTitle", {
                defaultValue: "Save your recovery codes",
              })}
              sub={t("auth.register.recoverySubtitle", {
                defaultValue: "Your last resort if you lose every other factor.",
              })}
            />
            <RecoveryCodesPanel
              codes={recoveryCodes}
              confirmLabel={t("auth.register.finish", {
                defaultValue: "Continue to Lumilio",
              })}
              checkboxLabel={t("auth.register.recoverySavedConfirm", {
                defaultValue: "I’ve saved my recovery codes somewhere safe",
              })}
              onConfirm={handleFinish}
            />
          </>
        )}
      </AuthShell>
    </div>
  );
};

export default RegistrationForm;
