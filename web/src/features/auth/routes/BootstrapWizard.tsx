import React, { useState, type FormEvent } from "react";
import {
  ArrowRight,
  Fingerprint,
  HardDrive,
  Image as ImageIcon,
  KeyRound,
  Server,
  ShieldCheck,
  Smartphone,
  Sparkles,
  UserCog,
  Users,
  User,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useRegistrationFlow } from "../hooks/useRegistrationFlow.ts";
import {
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "../lib/credentialPolicy.ts";
import {
  Brand,
  Btn,
  CardHead,
  Field,
  InlineError,
  PasskeyAffordance,
  PasswordField,
  RecoveryCodesPanel,
  Stepper,
  TextInput,
  TotpSetupPanel,
  type StepperStep,
} from "../components/ui.tsx";

const STEPPER: StepperStep[] = [
  { key: "welcome", label: "Welcome", icon: Server },
  { key: "admin", label: "Admin account", icon: UserCog },
  { key: "passkey", label: "Passkey", icon: Fingerprint },
  { key: "totp", label: "Authenticator", icon: Smartphone },
  { key: "recovery", label: "Recovery codes", icon: KeyRound },
];

const FLOW_INDEX: Record<string, number> = {
  credentials: 1,
  choose: 2,
  totp: 3,
  recovery: 4,
};

const BootstrapWizard: React.FC = () => {
  const { t } = useI18n();
  const [welcomed, setWelcomed] = useState(false);
  const {
    step,
    username,
    setUsername,
    password,
    setPassword,
    confirmPassword,
    setConfirmPassword,
    confirmPasswordRef,
    capabilityMessage,
    totpSetup,
    totpCode,
    setTotpCode,
    recoveryCodes,
    displayError,
    isBusy,
    handleStartRegistration,
    handleCreatePasskey,
    handleUseAuthenticatorApp,
    handleCompleteTotp,
    handleFinish,
  } = useRegistrationFlow();

  const appName = t("app.name", { defaultValue: "Lumilio" });
  const current = welcomed ? (FLOW_INDEX[step] ?? 1) : 0;
  const localizedSteps = STEPPER.map((s) => ({
    ...s,
    label: t(`auth.bootstrap.step.${s.key}`, { defaultValue: s.label }),
  }));

  const submitTotp = () => {
    void handleCompleteTotp({
      preventDefault: () => undefined,
    } as FormEvent<HTMLFormElement>);
  };

  const features: Array<[typeof ShieldCheck, string, string]> = [
    [
      ShieldCheck,
      t("auth.bootstrap.welcome.secureTitle", {
        defaultValue: "Secured by default",
      }),
      t("auth.bootstrap.welcome.secureBody", {
        defaultValue: "Passkey + authenticator required for the admin.",
      }),
    ],
    [
      HardDrive,
      t("auth.bootstrap.welcome.localTitle", {
        defaultValue: "Your data stays home",
      }),
      t("auth.bootstrap.welcome.localBody", {
        defaultValue: "Everything is stored on this server only.",
      }),
    ],
    [
      Users,
      t("auth.bootstrap.welcome.inviteTitle", {
        defaultValue: "Invite others later",
      }),
      t("auth.bootstrap.welcome.inviteBody", {
        defaultValue: "Add household members once setup is complete.",
      }),
    ],
  ];

  return (
    <div className="grid min-h-screen place-items-center bg-base-200 px-4 py-10">
      <div className="screen-in w-full" style={{ maxWidth: 880 }} key={`bootstrap-${current}`}>
        <div className="overflow-hidden rounded-2xl border border-base-200 bg-base-100 shadow-[0_1px_2px_rgba(0,0,0,0.04),0_18px_44px_-18px_rgba(0,0,0,0.18)] md:grid md:grid-cols-[260px_1fr]">
          {/* sidebar */}
          <aside className="hidden flex-col justify-between border-r border-base-200 bg-base-200/30 p-6 md:flex">
            <div>
              <Brand appName={appName} size={32} />
              <div className="mt-1.5 inline-flex items-center gap-1.5 rounded-full bg-base-200 px-2.5 py-1 text-[0.68rem] font-medium uppercase tracking-wide text-base-content/55">
                <Sparkles size={12} />{" "}
                {t("auth.bootstrap.firstRun", { defaultValue: "First-run setup" })}
              </div>
              <div className="mt-7">
                <Stepper steps={localizedSteps} current={current} />
              </div>
            </div>
            <p className="mt-8 text-xs leading-relaxed text-base-content/40">
              {t("auth.bootstrap.sidebarNote", {
                defaultValue:
                  "This wizard appears only once — until the first administrator is created.",
              })}
            </p>
          </aside>

          {/* content */}
          <section className="p-7 sm:p-9">
            {displayError && (
              <div className="mb-5">
                <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
              </div>
            )}

            {current === 0 && (
              <div className="flex max-w-md flex-col gap-5">
                <div className="grid h-12 w-12 place-items-center rounded-xl bg-base-200 text-base-content">
                  <Server size={24} />
                </div>
                <div>
                  <h1 className="text-2xl font-semibold tracking-tight text-base-content">
                    {t("auth.bootstrap.welcome.title", {
                      defaultValue: "Welcome to Lumilio Photos",
                    })}
                  </h1>
                  <p className="mt-2 text-sm leading-relaxed text-base-content/55">
                    {t("auth.bootstrap.welcome.body", {
                      defaultValue:
                        "Your server is up and running, but no administrator exists yet. Let’s create the first admin account and lock it down with two-factor authentication.",
                    })}
                  </p>
                </div>
                <dl className="grid gap-2.5">
                  {features.map(([Icon, title, body]) => (
                    <div
                      key={title}
                      className="flex items-start gap-3 rounded-xl border border-base-200 px-4 py-3"
                    >
                      <Icon size={18} className="mt-0.5 shrink-0 text-base-content/45" />
                      <div>
                        <p className="text-sm font-medium text-base-content">{title}</p>
                        <p className="text-xs text-base-content/55">{body}</p>
                      </div>
                    </div>
                  ))}
                </dl>
                <div className="flex items-center gap-3 rounded-xl bg-base-200/50 px-4 py-3 text-sm text-base-content/65">
                  <Server size={16} className="shrink-0 text-base-content/40" />
                  <span className="font-mono text-xs">
                    {typeof window !== "undefined" ? window.location.host : ""}
                  </span>
                </div>
                <Btn
                  variant="neutral"
                  icon={ArrowRight}
                  className="self-start px-6"
                  onClick={() => setWelcomed(true)}
                >
                  {t("auth.bootstrap.welcome.cta", { defaultValue: "Begin setup" })}
                </Btn>
              </div>
            )}

            {current === 1 && (
              <div className="max-w-md">
                <CardHead
                  title={t("auth.bootstrap.admin.title", {
                    defaultValue: "Create the administrator",
                  })}
                  sub={t("auth.bootstrap.admin.subtitle", {
                    defaultValue: "This account can manage the server, libraries, and members.",
                  })}
                />
                <form className="mt-5 flex flex-col gap-4" onSubmit={handleStartRegistration}>
                  <Field
                    label={t("auth.register.username", {
                      defaultValue: "Admin username",
                    })}
                    hint={t("auth.register.usernameHint", {
                      defaultValue: USERNAME_HINT,
                    })}
                  >
                    <TextInput
                      icon={User}
                      type="text"
                      placeholder={t("auth.register.usernamePlaceholder", {
                        defaultValue: "admin",
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
                  <div className="grid gap-4 sm:grid-cols-2">
                    <PasswordField
                      label={t("auth.register.password", {
                        defaultValue: "Password",
                      })}
                      value={password}
                      onChange={setPassword}
                      meter
                      autoComplete="new-password"
                    />
                    <PasswordField
                      label={t("auth.register.confirmPassword", {
                        defaultValue: "Confirm",
                      })}
                      value={confirmPassword}
                      onChange={setConfirmPassword}
                      placeholder={t("auth.register.confirmPasswordPlaceholder", {
                        defaultValue: "Re-enter",
                      })}
                      autoComplete="new-password"
                      inputRef={confirmPasswordRef}
                    />
                  </div>
                  <Btn type="submit" variant="neutral" loading={isBusy} className="self-start px-6">
                    {t("auth.bootstrap.admin.submit", {
                      defaultValue: "Create admin & continue",
                    })}
                  </Btn>
                </form>
              </div>
            )}

            {current === 2 && (
              <div className="max-w-md">
                <CardHead
                  icon={Fingerprint}
                  tone="primary"
                  title={t("auth.bootstrap.passkey.title", {
                    defaultValue: "Secure the admin with a passkey",
                  })}
                  sub={t("auth.bootstrap.passkey.subtitle", {
                    defaultValue:
                      "Strongly recommended — the admin account controls the whole server.",
                  })}
                />
                <div className="mt-5 flex flex-col gap-4">
                  {capabilityMessage && (
                    <div className="rounded-xl border border-base-200 bg-base-200/60 px-4 py-3 text-sm text-base-content/70">
                      {t(capabilityMessage, { defaultValue: capabilityMessage })}
                    </div>
                  )}
                  <PasskeyAffordance />
                  <Btn
                    variant="primary"
                    icon={Fingerprint}
                    loading={isBusy}
                    onClick={() => void handleCreatePasskey()}
                  >
                    {t("auth.bootstrap.passkey.action", {
                      defaultValue: "Create admin passkey",
                    })}
                  </Btn>
                </div>
                <button
                  type="button"
                  onClick={() => void handleUseAuthenticatorApp()}
                  disabled={isBusy}
                  className="mt-3 text-sm font-medium text-base-content/45 hover:text-base-content/70"
                >
                  {t("auth.bootstrap.passkey.skip", {
                    defaultValue: "Skip — use authenticator only",
                  })}
                </button>
              </div>
            )}

            {current === 3 && totpSetup && (
              <div className="max-w-md">
                <CardHead
                  icon={Smartphone}
                  tone="primary"
                  title={t("auth.bootstrap.totp.title", {
                    defaultValue: "Add an authenticator app",
                  })}
                  sub={t("auth.bootstrap.totp.subtitle", {
                    defaultValue: "Required for the admin account. Scan and verify to continue.",
                  })}
                />
                <div className="mt-5 flex flex-col gap-4">
                  <TotpSetupPanel
                    otpauthUri={totpSetup.otpauth_uri ?? ""}
                    secret={totpSetup.secret ?? ""}
                    code={totpCode}
                    onCodeChange={setTotpCode}
                    onVerify={submitTotp}
                    invalid={Boolean(displayError)}
                    busy={isBusy}
                    verifyLabel={t("auth.register.verifyAndFinish", {
                      defaultValue: "Verify & enable",
                    })}
                  />
                </div>
              </div>
            )}

            {current === 4 && (
              <div className="max-w-md">
                <CardHead
                  icon={KeyRound}
                  tone="warning"
                  title={t("auth.bootstrap.recovery.title", {
                    defaultValue: "Save admin recovery codes",
                  })}
                  sub={t("auth.bootstrap.recovery.subtitle", {
                    defaultValue: "The only way to recover the server if every factor is lost.",
                  })}
                />
                <div className="mt-5 flex flex-col gap-4">
                  <RecoveryCodesPanel
                    codes={recoveryCodes}
                    confirmLabel={t("auth.bootstrap.recovery.cta", {
                      defaultValue: "Finish & open dashboard",
                    })}
                    checkboxLabel={t("auth.register.recoverySavedConfirm", {
                      defaultValue: "I’ve saved my recovery codes somewhere safe",
                    })}
                    onConfirm={handleFinish}
                  />
                </div>
              </div>
            )}

            {/* mobile progress */}
            <div className="mt-7 flex items-center gap-1.5 md:hidden">
              {STEPPER.map((s, i) => (
                <span
                  key={s.key}
                  className={
                    i <= current
                      ? "h-1 grow rounded-full bg-neutral"
                      : "h-1 grow rounded-full bg-base-200"
                  }
                />
              ))}
            </div>
          </section>
        </div>
        <div className="mt-3 flex items-center justify-center gap-1.5 text-xs text-base-content/40">
          <ImageIcon size={12} /> {appName} ·{" "}
          {t("auth.common.selfHosted", { defaultValue: "Self-hosted" })}
        </div>
      </div>
    </div>
  );
};

export default BootstrapWizard;
