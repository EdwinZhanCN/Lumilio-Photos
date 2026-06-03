import React, { useState, type FormEvent } from "react";
import {
  ArrowRight,
  Fingerprint,
  HardDrive,
  KeyRound,
  Server,
  ShieldCheck,
  Smartphone,
  Sparkles,
  UserCog,
  Users,
  User,
  type LucideIcon,
} from "lucide-react";
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

const STEP_CONFIG: Array<{ key: string; icon: LucideIcon }> = [
  { key: "welcome", icon: Server },
  { key: "admin", icon: UserCog },
  { key: "totp", icon: Smartphone },
  { key: "passkey", icon: Fingerprint },
  { key: "recovery", icon: KeyRound },
];

const FLOW_INDEX: Record<string, number> = {
  credentials: 1,
  totp: 2,
  passkey: 3,
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
  const current = welcomed ? (FLOW_INDEX[step] ?? 1) : 0;
  const localizedSteps: StepperStep[] = [
    {
      key: "welcome",
      label: t("auth.bootstrap.step.welcome", { defaultValue: "Welcome" }),
      icon: Server,
    },
    {
      key: "admin",
      label: t("auth.bootstrap.step.admin", {
        defaultValue: "Admin account",
      }),
      icon: UserCog,
    },
    {
      key: "totp",
      label: t("auth.bootstrap.step.totp", {
        defaultValue: "Authenticator",
      }),
      icon: Smartphone,
    },
    {
      key: "passkey",
      label: t("auth.bootstrap.step.passkey", {
        defaultValue: "Passkey",
      }),
      icon: Fingerprint,
    },
    {
      key: "recovery",
      label: t("auth.bootstrap.step.recovery", {
        defaultValue: "Recovery codes",
      }),
      icon: KeyRound,
    },
  ];

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
        defaultValue: "Passkey or authenticator app — pick one as the second factor.",
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
                {t("auth.bootstrap.firstRun", {
                  defaultValue: "First-run setup",
                })}
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
                  {t("auth.bootstrap.welcome.cta", {
                    defaultValue: "Begin setup",
                  })}
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
                  <PasswordField
                    label={t("auth.register.password", {
                      defaultValue: "Password",
                    })}
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
                  <Btn type="submit" variant="neutral" loading={isBusy} className="self-start px-6">
                    {t("auth.bootstrap.admin.submit", {
                      defaultValue: "Create admin & continue",
                    })}
                  </Btn>
                </form>
              </div>
            )}

            {current === 2 && totpSetup && (
              <div className="max-w-md">
                <CardHead
                  icon={Smartphone}
                  tone="primary"
                  title={t("auth.bootstrap.totp.title", {
                    defaultValue: "Add an authenticator app",
                  })}
                  sub={t("auth.bootstrap.totp.subtitle", {
                    defaultValue: "Optional — scan and verify to enable an authenticator app.",
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
                    verifyLabel={t("auth.register.verifyAndEnable", {
                      defaultValue: "Verify & enable",
                    })}
                  />
                </div>
                <button
                  type="button"
                  onClick={handleSkipTotp}
                  disabled={isBusy}
                  className="mt-3 text-sm font-medium text-base-content/45 hover:text-base-content/70"
                >
                  {t("auth.bootstrap.totp.skip", {
                    defaultValue: "Skip for now",
                  })}
                </button>
              </div>
            )}

            {current === 3 && (
              <div className="max-w-md">
                <CardHead
                  icon={Fingerprint}
                  tone="primary"
                  title={t("auth.bootstrap.passkey.title", {
                    defaultValue: "Add an admin passkey",
                  })}
                  sub={t("auth.bootstrap.passkey.subtitle", {
                    defaultValue: "Optional — a fast, phishing-resistant way to sign in as admin.",
                  })}
                />
                <div className="mt-5 flex flex-col gap-4">
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
                  onClick={handleSkipPasskey}
                  disabled={isBusy}
                  className="mt-3 text-sm font-medium text-base-content/45 hover:text-base-content/70"
                >
                  {t("auth.bootstrap.passkey.skip", {
                    defaultValue: "Skip for now",
                  })}
                </button>
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
              {STEP_CONFIG.map((s, i) => (
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
      </div>
    </div>
  );
};

export default BootstrapWizard;
