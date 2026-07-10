import React, { useEffect, useMemo, useState, type FormEvent } from "react";
import {
  ArrowRight,
  CheckCircle,
  Fingerprint,
  FolderPlus,
  Globe,
  HardDrive,
  KeyRound,
  Server,
  ShieldCheck,
  Smartphone,
  Sparkles,
  UserCog,
  Users,
  User,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { SUPPORTED_LANGUAGES } from "@/lib/i18n.tsx";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "@/features/settings/preferences.ts";
import { useRegistrationFlow } from "../hooks/useRegistrationFlow.ts";
import { setupStatusQueryKey, useSetupStatus } from "../hooks/useSetupStatus.ts";
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

const LANGUAGE_OPTIONS: { value: string; label: string }[] = [
  { value: "en", label: "English" },
  { value: "zh", label: "中文" },
];

const REGION_OPTIONS_KEYS = [
  { value: "china", labelKey: "settings.regionOptions.china", fallback: "China" },
  { value: "other", labelKey: "settings.regionOptions.other", fallback: "Other" },
] as const;

const FLOW_INDEX: Record<string, number> = {
  credentials: 1,
  totp: 2,
  passkey: 3,
  recovery: 4,
};

const isStorageStrategy = (value?: string): value is "cas" | "date" | "flat" =>
  value === "cas" || value === "date" || value === "flat";
const isDuplicateHandling = (value?: string): value is "overwrite" | "rename" | "uuid" =>
  value === "overwrite" || value === "rename" || value === "uuid";

function apiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const record = error as { message?: string; error?: string };
    return record.message || record.error || fallback;
  }
  return fallback;
}

const BootstrapWizard: React.FC = () => {
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const createRepoMutation = $api.useMutation("post", "/api/v1/repositories");

  const [welcomed, setWelcomed] = useState(false);
  const [mfaComplete, setMfaComplete] = useState(false);
  const [language, setLanguage] = usePreference("language");
  const [region, setRegion] = usePreference("region");

  // Repository form state
  const defaults = setupQuery.data?.repository_defaults;
  const [repoName, setRepoName] = useState("Primary Storage");
  const [repoRoot, setRepoRoot] = useState("");
  const [strategy, setStrategy] = useState<"cas" | "date" | "flat">("date");
  const [duplicateHandling, setDuplicateHandling] = useState<"overwrite" | "rename" | "uuid">(
    "rename",
  );

  useEffect(() => {
    if (!defaults) return;
    setRepoRoot((current) => current || defaults.default_root || "");
    setStrategy(isStorageStrategy(defaults.strategy) ? defaults.strategy : "date");
    setDuplicateHandling(
      isDuplicateHandling(defaults.duplicate_handling) ? defaults.duplicate_handling : "rename",
    );
  }, [defaults]);

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
  } = useRegistrationFlow({ onComplete: () => setMfaComplete(true) });

  const appName = t("app.name", { defaultValue: "Lumilio Photos" });

  // Step mapping: 0=welcome, 1-4=registration flow, 5=repository
  const current = mfaComplete ? 5 : welcomed ? (FLOW_INDEX[step] ?? 1) : 0;

  const localizedSteps: StepperStep[] = [
    {
      key: "welcome",
      label: t("auth.bootstrap.step.welcome", { defaultValue: "Welcome" }),
      icon: Globe,
    },
    {
      key: "admin",
      label: t("auth.bootstrap.step.admin", { defaultValue: "Admin account" }),
      icon: UserCog,
    },
    {
      key: "totp",
      label: t("auth.bootstrap.step.totp", { defaultValue: "Authenticator" }),
      icon: Smartphone,
    },
    {
      key: "passkey",
      label: t("auth.bootstrap.step.passkey", { defaultValue: "Passkey" }),
      icon: Fingerprint,
    },
    {
      key: "recovery",
      label: t("auth.bootstrap.step.recovery", { defaultValue: "Recovery codes" }),
      icon: KeyRound,
    },
    {
      key: "repository",
      label: t("auth.bootstrap.step.repository", { defaultValue: "Storage" }),
      icon: HardDrive,
    },
  ];

  const submitTotp = () => {
    void handleCompleteTotp({
      preventDefault: () => undefined,
    } as FormEvent<HTMLFormElement>);
  };

  const canSubmitRepo = useMemo(
    () => repoName.trim() !== "" && repoRoot.trim() !== "" && !createRepoMutation.isPending,
    [createRepoMutation.isPending, repoName, repoRoot],
  );

  const submitRepo = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitRepo) return;
    await createRepoMutation.mutateAsync({
      body: {
        name: repoName.trim(),
        role: "primary",
        root: repoRoot.trim(),
        storage_strategy: strategy,
        duplicate_handling: duplicateHandling,
      },
    });
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: setupStatusQueryKey }),
      queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/assets/indexing/repositories"] }),
    ]);
    void navigate("/", { replace: true });
  };

  const repoError = createRepoMutation.error
    ? apiMessage(
        createRepoMutation.error,
        t("auth.primaryRepository.error", {
          defaultValue: "Failed to create the primary repository.",
        }),
      )
    : null;

  const features: Array<[typeof ShieldCheck, string, string]> = [
    [
      ShieldCheck,
      t("auth.bootstrap.welcome.secureTitle", { defaultValue: "Secured by default" }),
      t("auth.bootstrap.welcome.secureBody", {
        defaultValue: "Passkey or authenticator app — pick one as the second factor.",
      }),
    ],
    [
      HardDrive,
      t("auth.bootstrap.welcome.localTitle", { defaultValue: "Your data stays home" }),
      t("auth.bootstrap.welcome.localBody", {
        defaultValue: "Everything is stored on this server only.",
      }),
    ],
    [
      Users,
      t("auth.bootstrap.welcome.inviteTitle", { defaultValue: "Invite others later" }),
      t("auth.bootstrap.welcome.inviteBody", {
        defaultValue: "Add household members once setup is complete.",
      }),
    ],
  ];

  return (
    <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
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
            {displayError && current < 5 && (
              <div className="mb-5">
                <InlineError>{t(displayError, { defaultValue: displayError })}</InlineError>
              </div>
            )}

            {/* Step 0: Welcome + Language/Region */}
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
                        "Your server is up and running, but no administrator exists yet. Let's create the first admin account and lock it down with two-factor authentication.",
                    })}
                  </p>
                </div>

                {/* Language & Region */}
                <div className="flex flex-col gap-3 rounded-xl border border-base-200 px-4 py-3.5">
                  <div className="flex items-center gap-2 text-sm font-medium text-base-content">
                    <Globe size={16} className="text-base-content/45" />
                    {t("auth.bootstrap.welcome.languageRegion", {
                      defaultValue: "Language & Region",
                    })}
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <label className="flex flex-col gap-1">
                      <span className="text-xs text-base-content/55">
                        {t("settings.language", { defaultValue: "Language" })}
                      </span>
                      <select
                        className="select select-bordered select-sm w-full"
                        value={language}
                        onChange={(e) =>
                          setLanguage(e.target.value as (typeof SUPPORTED_LANGUAGES)[number])
                        }
                      >
                        {LANGUAGE_OPTIONS.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {opt.label}
                          </option>
                        ))}
                      </select>
                    </label>
                    <label className="flex flex-col gap-1">
                      <span className="text-xs text-base-content/55">
                        {t("settings.region", { defaultValue: "Region" })}
                      </span>
                      <select
                        className="select select-bordered select-sm w-full"
                        value={region}
                        onChange={(e) => setRegion(e.target.value as "china" | "other")}
                      >
                        {REGION_OPTIONS_KEYS.map((opt) => (
                          <option key={opt.value} value={opt.value}>
                            {t(opt.labelKey, { defaultValue: opt.fallback })}
                          </option>
                        ))}
                      </select>
                    </label>
                  </div>
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

            {/* Step 1: Admin account */}
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
                    label={t("auth.register.username", { defaultValue: "Admin username" })}
                    hint={t("auth.register.usernameHint", { defaultValue: USERNAME_HINT })}
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
                    label={t("auth.register.password", { defaultValue: "Password" })}
                    hint={t("auth.register.passwordHint", { defaultValue: PASSWORD_HINT })}
                    value={password}
                    onChange={setPassword}
                    meter
                    autoComplete="new-password"
                  />
                  <PasswordField
                    label={t("auth.register.confirmPassword", { defaultValue: "Confirm password" })}
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
                    {t("auth.bootstrap.admin.submit", { defaultValue: "Create admin & continue" })}
                  </Btn>
                </form>
              </div>
            )}

            {/* Step 2: TOTP */}
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
                  {t("auth.bootstrap.totp.skip", { defaultValue: "Skip for now" })}
                </button>
              </div>
            )}

            {/* Step 3: Passkey */}
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
                    {t("auth.bootstrap.passkey.action", { defaultValue: "Create admin passkey" })}
                  </Btn>
                </div>
                <button
                  type="button"
                  onClick={handleSkipPasskey}
                  disabled={isBusy}
                  className="mt-3 text-sm font-medium text-base-content/45 hover:text-base-content/70"
                >
                  {t("auth.bootstrap.passkey.skip", { defaultValue: "Skip for now" })}
                </button>
              </div>
            )}

            {/* Step 4: Recovery codes */}
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
                      defaultValue: "Continue to storage setup",
                    })}
                    checkboxLabel={t("auth.register.recoverySavedConfirm", {
                      defaultValue: "I've saved my recovery codes somewhere safe",
                    })}
                    onConfirm={handleFinish}
                  />
                </div>
              </div>
            )}

            {/* Step 5: Primary repository */}
            {current === 5 && (
              <div className="max-w-md">
                <CardHead
                  icon={HardDrive}
                  title={t("auth.bootstrap.repository.title", {
                    defaultValue: "Set up primary storage",
                  })}
                  sub={t("auth.bootstrap.repository.subtitle", {
                    defaultValue:
                      "Choose where Lumilio stores photos. This becomes the default for future repositories.",
                  })}
                />

                {repoError && (
                  <div className="mt-4">
                    <InlineError>{repoError}</InlineError>
                  </div>
                )}

                <form className="mt-5 flex flex-col gap-4" onSubmit={(e) => void submitRepo(e)}>
                  <Field label={t("auth.primaryRepository.name", { defaultValue: "Name" })}>
                    <TextInput
                      icon={FolderPlus}
                      type="text"
                      value={repoName}
                      onChange={(e) => setRepoName(e.target.value)}
                      disabled={createRepoMutation.isPending}
                      required
                    />
                  </Field>

                  <Field
                    label={t("auth.primaryRepository.root", { defaultValue: "Storage root" })}
                    hint={t("auth.primaryRepository.rootHint", {
                      defaultValue:
                        "Set by server configuration. The primary repository is created at <root>/primary.",
                    })}
                  >
                    <TextInput
                      icon={HardDrive}
                      type="text"
                      value={repoRoot}
                      readOnly
                      tabIndex={-1}
                      className="bg-base-200 font-mono text-sm"
                    />
                  </Field>

                  <div className="grid grid-cols-2 gap-3">
                    <label className="flex flex-col gap-1">
                      <span className="text-xs font-medium text-base-content/70">
                        {t("auth.primaryRepository.strategy", { defaultValue: "Storage strategy" })}
                      </span>
                      <select
                        className="select select-bordered select-sm w-full"
                        value={strategy}
                        onChange={(e) => {
                          if (isStorageStrategy(e.target.value)) setStrategy(e.target.value);
                        }}
                        disabled={createRepoMutation.isPending}
                      >
                        <option value="date">date</option>
                        <option value="flat">flat</option>
                        <option value="cas">cas</option>
                      </select>
                    </label>
                    <label className="flex flex-col gap-1">
                      <span className="text-xs font-medium text-base-content/70">
                        {t("auth.primaryRepository.duplicates", { defaultValue: "Duplicates" })}
                      </span>
                      <select
                        className="select select-bordered select-sm w-full"
                        value={duplicateHandling}
                        onChange={(e) => {
                          if (isDuplicateHandling(e.target.value)) {
                            setDuplicateHandling(e.target.value);
                          }
                        }}
                        disabled={createRepoMutation.isPending}
                      >
                        <option value="rename">rename</option>
                        <option value="uuid">uuid</option>
                        <option value="overwrite">overwrite</option>
                      </select>
                    </label>
                  </div>

                  <Btn
                    type="submit"
                    variant="neutral"
                    icon={CheckCircle}
                    loading={createRepoMutation.isPending}
                    disabled={!canSubmitRepo}
                    className="self-start px-6"
                  >
                    {t("auth.bootstrap.repository.submit", {
                      defaultValue: "Create & open dashboard",
                    })}
                  </Btn>
                </form>
              </div>
            )}

            {/* mobile progress */}
            <div className="mt-7 flex items-center gap-1.5 md:hidden">
              {localizedSteps.map((s, i) => (
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
