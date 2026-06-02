import React, {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import QRCode from "qrcode";
import { useI18n } from "@/lib/i18n.tsx";
import {
  ArrowRight,
  Check,
  CircleAlert,
  Copy,
  Download,
  Eye,
  EyeOff,
  Fingerprint,
  Info,
  KeyRound,
  Lock,
  ScanFace,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";

/**
 * Shared authentication UI primitives.
 *
 * These mirror the Lumilio auth design handoff (daisyUI semantic roles mapped
 * onto the app's own `lumilio` theme). They are presentational only — every
 * API call stays in the feature hooks/pages so the same kit serves login,
 * register, bootstrap, and security-settings flows.
 */

export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

/* ---------------------------------------------------------------- brand --- */

export const Brand: React.FC<{
  appName?: string;
  size?: number;
  withWord?: boolean;
  className?: string;
}> = ({
  appName = "Lumilio Photos",
  size = 36,
  withWord = true,
  className,
}) => (
  <div className={cx("flex items-center gap-2.5", className)}>
    <img
      src="/logo.png"
      alt={`${appName} logo`}
      className="shrink-0 object-contain"
      style={{ width: size, height: size }}
    />
    {withWord && (
      <span className="text-[1.35rem] font-semibold tracking-tight text-base-content">
        {appName}
      </span>
    )}
  </div>
);

/* ----------------------------------------------------------- card shell --- */

export const AuthShell: React.FC<{
  children: ReactNode;
  width?: number;
  appName?: string;
}> = ({ children, width = 440, appName }) => (
  <div className="screen-in w-full" style={{ maxWidth: width }}>
    <div className="mb-7 flex justify-center">
      <Brand appName={appName} />
    </div>
    <div className="card border border-base-200 bg-base-100 shadow-[0_1px_2px_rgba(0,0,0,0.04),0_12px_32px_-12px_rgba(0,0,0,0.12)]">
      <div className="card-body gap-5 p-7 sm:p-8">{children}</div>
    </div>
  </div>
);

export type HeadTone = "neutral" | "primary" | "success" | "warning";

const HEAD_TONES: Record<HeadTone, string> = {
  neutral: "bg-base-200 text-base-content",
  primary: "bg-primary/10 text-primary",
  success: "bg-success/15 text-success",
  warning: "bg-warning/15 text-warning",
};

export const CardHead: React.FC<{
  icon?: LucideIcon;
  title: ReactNode;
  sub?: ReactNode;
  tone?: HeadTone;
}> = ({ icon: Icon, title, sub, tone = "neutral" }) => (
  <div className="flex flex-col gap-3">
    {Icon && (
      <div
        className={cx(
          "grid h-11 w-11 place-items-center rounded-xl",
          HEAD_TONES[tone],
        )}
      >
        <Icon size={22} />
      </div>
    )}
    <div>
      <h1 className="text-[1.4rem] font-semibold leading-tight tracking-tight text-base-content">
        {title}
      </h1>
      {sub && (
        <p className="mt-1.5 text-sm leading-relaxed text-base-content/55">
          {sub}
        </p>
      )}
    </div>
  </div>
);

/* --------------------------------------------------------------- fields --- */

export const Field: React.FC<{
  label?: ReactNode;
  hint?: string;
  error?: ReactNode;
  htmlFor?: string;
  children: ReactNode;
}> = ({ label, hint, error, htmlFor, children }) => {
  return (
    <div className="form-control w-full min-w-0">
      {(label || hint) && (
        <div className="mb-1.5 flex min-w-0 items-center gap-1.5">
          {label && (
            <label
              className="min-w-0 text-sm font-medium text-base-content/80"
              htmlFor={htmlFor}
            >
              {label}
            </label>
          )}
          {hint && (
            <div className="tooltip tooltip-right inline-flex" data-tip={hint}>
              <button
                type="button"
                className="btn btn-ghost btn-xs h-5 min-h-0 w-5 shrink-0 rounded-full p-0 text-base-content/40 hover:bg-base-200 hover:text-base-content/70"
                aria-label={hint}
              >
                <Info size={13} />
              </button>
            </div>
          )}
        </div>
      )}
      {children}
      {error && (
        <span className="mt-1.5 flex items-center gap-1 text-xs font-medium text-error">
          <CircleAlert size={13} /> {error}
        </span>
      )}
    </div>
  );
};

type TextInputProps = React.InputHTMLAttributes<HTMLInputElement> & {
  icon?: LucideIcon;
  invalid?: boolean;
};

export const TextInput = React.forwardRef<HTMLInputElement, TextInputProps>(
  function TextInput({ icon: Icon, invalid, className, ...props }, ref) {
    if (Icon) {
      return (
        <div
          className={cx(
            "input input-bordered flex w-full min-w-0 items-center gap-2.5 bg-base-100",
            invalid && "input-error",
            className,
          )}
        >
          <Icon size={17} className="shrink-0 text-base-content/40" />
          <input
            ref={ref}
            className="min-w-0 grow bg-transparent outline-none placeholder:text-base-content/35"
            {...props}
          />
        </div>
      );
    }
    return (
      <input
        ref={ref}
        className={cx(
          "input input-bordered w-full min-w-0 bg-base-100",
          invalid && "input-error",
          className,
        )}
        {...props}
      />
    );
  },
);

/* ------------------------------------------------------------- password --- */

export type PasswordStrength = { score: number; label: string };

const STRENGTH_LABELS = ["Too short", "Weak", "Fair", "Good", "Strong"];

export function passwordStrength(pw: string): PasswordStrength {
  if (!pw) return { score: 0, label: "" };
  let s = 0;
  if (pw.length >= 8) s++;
  if (pw.length >= 12) s++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) s++;
  if (/\d/.test(pw)) s++;
  if (/[^A-Za-z0-9]/.test(pw)) s++;
  s = Math.min(s, 4);
  return { score: s, label: STRENGTH_LABELS[s] };
}

const STRENGTH_COLORS = [
  "bg-base-300",
  "bg-error",
  "bg-warning",
  "bg-success/70",
  "bg-success",
];

type PasswordFieldProps = {
  label?: ReactNode;
  hint?: string;
  error?: ReactNode;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  autoComplete?: string;
  meter?: boolean;
  inputRef?: React.Ref<HTMLInputElement>;
  strengthLabels?: string[];
};

export const PasswordField: React.FC<PasswordFieldProps> = ({
  label = "Password",
  hint,
  error,
  value,
  onChange,
  placeholder = "••••••••",
  autoComplete = "new-password",
  meter = false,
  inputRef,
  strengthLabels,
}) => {
  const [show, setShow] = useState(false);
  const st = passwordStrength(value);
  const label_ =
    strengthLabels && st.score > 0 ? strengthLabels[st.score] : st.label;
  return (
    <Field label={label} hint={hint} error={error}>
      <div
        className={cx(
          "input input-bordered flex w-full min-w-0 items-center gap-2.5 bg-base-100",
          !!error && "input-error",
        )}
      >
        <Lock size={17} className="shrink-0 text-base-content/40" />
        <input
          ref={inputRef}
          type={show ? "text" : "password"}
          className="min-w-0 grow bg-transparent outline-none placeholder:text-base-content/35"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          autoComplete={autoComplete}
        />
        <button
          type="button"
          onClick={() => setShow((s) => !s)}
          className="text-base-content/40 transition hover:text-base-content/70"
          tabIndex={-1}
          aria-label={show ? "Hide password" : "Show password"}
        >
          {show ? <EyeOff size={17} /> : <Eye size={17} />}
        </button>
      </div>
      {meter && value && (
        <div className="mt-2 flex items-center gap-2">
          <div className="flex grow gap-1">
            {[1, 2, 3, 4].map((i) => (
              <span
                key={i}
                className={cx(
                  "h-1 grow rounded-full transition-colors",
                  i <= st.score ? STRENGTH_COLORS[st.score] : "bg-base-300",
                )}
              />
            ))}
          </div>
          <span className="w-12 text-right text-xs font-medium text-base-content/50">
            {label_}
          </span>
        </div>
      )}
    </Field>
  );
};

/* --------------------------------------------------------------- button --- */

export type BtnVariant = "primary" | "neutral" | "outline" | "ghost";

const BTN_VARIANTS: Record<BtnVariant, string> = {
  primary: "btn-primary",
  neutral: "btn-neutral",
  outline:
    "btn-outline border-base-300 hover:border-base-content/30 hover:bg-base-200 text-base-content",
  ghost: "btn-ghost text-base-content/70",
};

type BtnProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: BtnVariant;
  loading?: boolean;
  icon?: LucideIcon;
};

export const Btn: React.FC<BtnProps> = ({
  children,
  variant = "primary",
  loading,
  icon: Icon,
  className,
  disabled,
  ...props
}) => (
  <button
    className={cx(
      "btn h-12 min-h-12 gap-2 rounded-xl text-[0.95rem] font-medium normal-case",
      BTN_VARIANTS[variant],
      className,
    )}
    disabled={loading || disabled}
    {...props}
  >
    {loading ? (
      <span className="loading loading-spinner loading-sm" />
    ) : (
      Icon && <Icon size={18} />
    )}
    {children}
  </button>
);

/* ------------------------------------------------------------ otp input --- */

export const OtpInput: React.FC<{
  length?: number;
  value: string;
  onChange: (value: string) => void;
  onComplete?: (value: string) => void;
  autoFocus?: boolean;
  invalid?: boolean;
}> = ({
  length = 6,
  value,
  onChange,
  onComplete,
  autoFocus = true,
  invalid,
}) => {
  const refs = useRef<Array<HTMLInputElement | null>>([]);
  const chars = Array.from({ length }, (_, i) => value[i] ?? "");

  useEffect(() => {
    if (autoFocus) refs.current[0]?.focus();
  }, [autoFocus]);

  const setAt = (i: number, ch: string) => {
    const next = (value.slice(0, i) + ch + value.slice(i + 1)).slice(0, length);
    onChange(next);
    return next;
  };

  const handle = (i: number, e: React.ChangeEvent<HTMLInputElement>) => {
    const v = e.target.value.replace(/\D/g, "");
    if (!v) return;
    const next = setAt(i, v[v.length - 1]);
    if (i < length - 1) refs.current[i + 1]?.focus();
    if (next.length === length) onComplete?.(next);
  };

  const key = (i: number, e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Backspace") {
      e.preventDefault();
      if (chars[i]) {
        setAt(i, "");
      } else if (i > 0) {
        refs.current[i - 1]?.focus();
        setAt(i - 1, "");
      }
    }
    if (e.key === "ArrowLeft" && i > 0) refs.current[i - 1]?.focus();
    if (e.key === "ArrowRight" && i < length - 1) refs.current[i + 1]?.focus();
  };

  const paste = (e: React.ClipboardEvent<HTMLDivElement>) => {
    e.preventDefault();
    const txt = (e.clipboardData.getData("text") || "")
      .replace(/\D/g, "")
      .slice(0, length);
    if (!txt) return;
    onChange(txt);
    refs.current[Math.min(txt.length, length - 1)]?.focus();
    if (txt.length === length) onComplete?.(txt);
  };

  return (
    <div className="flex justify-between gap-2" onPaste={paste}>
      {chars.map((c, i) => (
        <input
          // eslint-disable-next-line react/no-array-index-key
          key={i}
          ref={(el) => {
            refs.current[i] = el;
          }}
          inputMode="numeric"
          autoComplete={i === 0 ? "one-time-code" : "off"}
          maxLength={1}
          value={c}
          onChange={(e) => handle(i, e)}
          onKeyDown={(e) => key(i, e)}
          className={cx(
            "h-14 w-full rounded-xl border bg-base-100 text-center text-2xl font-semibold tabular-nums text-base-content outline-none transition",
            invalid
              ? "border-error"
              : "border-base-300 focus:border-primary focus:ring-2 focus:ring-primary/15",
          )}
        />
      ))}
    </div>
  );
};

/* ------------------------------------------------------------------ qr --- */

export const AuthQR: React.FC<{ value: string; size?: number }> = ({
  value,
  size = 168,
}) => {
  const [dataUrl, setDataUrl] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    if (!value) {
      setDataUrl(null);
      return;
    }
    QRCode.toDataURL(value, {
      errorCorrectionLevel: "M",
      margin: 1,
      width: size * 2,
      color: { dark: "#111111", light: "#ffffff" },
    })
      .then((url) => {
        if (!cancelled) setDataUrl(url);
      })
      .catch(() => {
        if (!cancelled) setDataUrl(null);
      });
    return () => {
      cancelled = true;
    };
  }, [value, size]);

  return (
    <div className="inline-block rounded-2xl border border-base-200 bg-white p-3.5 shadow-sm">
      {dataUrl ? (
        <img
          src={dataUrl}
          alt="Authenticator QR code"
          width={size}
          height={size}
        />
      ) : (
        <div
          className="grid place-items-center text-base-content/30"
          style={{ width: size, height: size }}
        >
          <span className="loading loading-spinner" />
        </div>
      )}
    </div>
  );
};

/* ----------------------------------------------------------------- copy --- */

export const CopyButton: React.FC<{
  text: string;
  label?: string;
  copiedLabel?: string;
  variant?: "chip" | "block";
}> = ({ text, label = "Copy", copiedLabel = "Copied", variant = "chip" }) => {
  const [done, setDone] = useState(false);
  const copy = () => {
    void navigator.clipboard?.writeText(text).catch(() => undefined);
    setDone(true);
    window.setTimeout(() => setDone(false), 1600);
  };
  if (variant === "block") {
    return (
      <button
        type="button"
        onClick={copy}
        className="btn btn-outline h-11 min-h-11 w-full gap-2 rounded-xl border-base-300 font-medium normal-case text-base-content hover:bg-base-200"
      >
        {done ? <Check size={16} /> : <Copy size={16} />}{" "}
        {done ? copiedLabel : label}
      </button>
    );
  }
  return (
    <button
      type="button"
      onClick={copy}
      className="btn btn-ghost btn-sm gap-1.5 rounded-lg text-base-content/60 hover:text-base-content"
    >
      {done ? <Check size={14} /> : <Copy size={14} />}{" "}
      {done ? copiedLabel : label}
    </button>
  );
};

/* ----------------------------------------------------------- flow steps --- */

export const FlowSteps: React.FC<{ steps: string[]; current: number }> = ({
  steps,
  current,
}) => (
  <ul className="mb-1 flex items-center gap-1.5 text-xs font-medium">
    {steps.map((s, i) => {
      const done = i < current;
      const active = i === current;
      return (
        <li key={s} className="flex items-center gap-1.5">
          <span
            className={cx(
              "grid h-5 w-5 place-items-center rounded-full text-[0.65rem]",
              done
                ? "bg-success text-success-content"
                : active
                  ? "bg-neutral text-neutral-content"
                  : "bg-base-200 text-base-content/40",
            )}
          >
            {done ? <Check size={11} /> : i + 1}
          </span>
          <span
            className={active ? "text-base-content" : "text-base-content/40"}
          >
            {s}
          </span>
          {i < steps.length - 1 && (
            <span className="mx-0.5 h-px w-4 bg-base-300" />
          )}
        </li>
      );
    })}
  </ul>
);

/* ------------------------------------------------------------- stepper --- */

export type StepperStep = { key: string; label: string; icon: LucideIcon };

export const Stepper: React.FC<{ steps: StepperStep[]; current: number }> = ({
  steps,
  current,
}) => {
  const { t } = useI18n();
  return (
    <ol className="flex flex-col gap-1">
      {steps.map((s, i) => {
        const done = i < current;
        const active = i === current;
        const Icon = s.icon;
        return (
          <li
            key={s.key}
            className="flex items-center gap-3 rounded-lg px-2.5 py-2"
          >
            <span
              className={cx(
                "grid h-8 w-8 shrink-0 place-items-center rounded-lg transition-colors",
                done
                  ? "bg-success/15 text-success"
                  : active
                    ? "bg-neutral text-neutral-content"
                    : "bg-base-200 text-base-content/35",
              )}
            >
              {done ? <Check size={16} /> : <Icon size={16} />}
            </span>
            <div className="leading-tight">
              <p
                className={cx(
                  "text-sm font-medium",
                  active
                    ? "text-base-content"
                    : done
                      ? "text-base-content/70"
                      : "text-base-content/40",
                )}
              >
                {s.label}
              </p>
              <p className="text-[0.7rem] text-base-content/35">
                {t("auth.bootstrap.stepNum", {
                  defaultValue: "Step {{n}}",
                  n: i + 1,
                })}
              </p>
            </div>
          </li>
        );
      })}
    </ol>
  );
};

/* --------------------------------------------------- passkey affordance --- */

const PASSKEY_METHODS: Array<[LucideIcon, string, string]> = [
  [ScanFace, "auth.passkey.affordance.method.face", "Face"],
  [Fingerprint, "auth.passkey.affordance.method.touch", "Touch"],
  [KeyRound, "auth.passkey.affordance.method.security_key", "Security key"],
];

export const PasskeyAffordance: React.FC<{
  created?: boolean;
  headline?: string;
  description?: string;
  activeLabel?: string;
}> = ({ created = false, headline, description, activeLabel }) => {
  const { t } = useI18n();
  const resolvedHeadline =
    headline ??
    t("auth.register.passkeyHeadline", {
      defaultValue: "Sign in with your face or fingerprint",
    });
  const resolvedDescription =
    description ??
    t("auth.register.passkeyDescription", {
      defaultValue:
        "No password to remember. Your passkey stays on this device and can’t be phished.",
    });
  const resolvedActiveLabel =
    activeLabel ??
    t("auth.passkey.affordance.activeLabel", {
      defaultValue: "Passkey active",
    });

  return (
    <>
      <div className="flex flex-col items-center gap-4 rounded-2xl border border-dashed border-base-300 bg-base-200/40 px-5 py-7 text-center">
        <div className="grid h-16 w-16 place-items-center rounded-2xl bg-base-100 shadow-sm">
          {created ? (
            <Check size={30} className="text-success" />
          ) : (
            <Fingerprint size={30} className="text-base-content" />
          )}
        </div>
        {created ? (
          <p className="font-medium text-base-content">{resolvedActiveLabel}</p>
        ) : (
          <div className="space-y-1">
            <p className="font-medium text-base-content">{resolvedHeadline}</p>
            <p className="text-sm leading-relaxed text-base-content/55">
              {resolvedDescription}
            </p>
          </div>
        )}
      </div>
      {!created && (
        <div className="grid grid-cols-3 gap-2">
          {PASSKEY_METHODS.map(([Icon, key, fallback]) => (
            <div
              key={key}
              className="flex flex-col items-center gap-1.5 rounded-xl border border-base-200 py-3 text-xs text-base-content/55"
            >
              <Icon size={18} className="text-base-content/45" />{" "}
              {t(key, { defaultValue: fallback })}
            </div>
          ))}
        </div>
      )}
    </>
  );
};

/* ------------------------------------------------------ recovery panel --- */

export const RecoveryCodesPanel: React.FC<{
  codes: string[];
  confirmLabel?: string;
  warning?: string;
  checkboxLabel?: string;
  onConfirm: () => void;
  busy?: boolean;
}> = ({
  codes,
  confirmLabel = "Continue",
  warning = "Each code works once. Store them somewhere safe — they’re the only way back in if you lose your passkey and authenticator.",
  checkboxLabel = "I’ve saved my recovery codes somewhere safe",
  onConfirm,
  busy,
}) => {
  const [saved, setSaved] = useState(false);
  const [pulled, setPulled] = useState(false);
  const download = () => {
    setPulled(true);
    try {
      const blob = new Blob(
        [
          `Lumilio Photos — recovery codes\nGenerated ${new Date().toLocaleString()}\n\n${codes.join(
            "\n",
          )}\n`,
        ],
        { type: "text/plain" },
      );
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = "lumilio-recovery-codes.txt";
      a.click();
      URL.revokeObjectURL(a.href);
    } catch {
      /* clipboard/blob unavailable */
    }
  };
  return (
    <>
      <div className="flex items-start gap-2.5 rounded-xl bg-warning/10 px-4 py-3 text-sm text-base-content/75">
        <TriangleAlert size={16} className="mt-0.5 shrink-0 text-warning" />
        <span>{warning}</span>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-2.5 rounded-2xl border border-base-200 bg-base-200/40 p-5 font-mono text-[0.92rem] tabular-nums text-base-content">
        {codes.map((c, i) => (
          <div key={c} className="flex items-center gap-2">
            <span className="w-4 text-right text-xs text-base-content/35">
              {i + 1}
            </span>
            <span className="tracking-tight">{c}</span>
          </div>
        ))}
      </div>
      <div className="flex gap-2">
        <button
          type="button"
          onClick={download}
          className="btn btn-outline h-11 min-h-11 flex-1 gap-2 rounded-xl border-base-300 font-medium normal-case text-base-content hover:bg-base-200"
        >
          {pulled ? <Check size={16} /> : <Download size={16} />}{" "}
          {pulled ? "Downloaded" : "Download"}
        </button>
        <div className="flex-1">
          <CopyButton
            text={codes.join("\n")}
            label="Copy all"
            variant="block"
          />
        </div>
      </div>
      <label className="flex cursor-pointer items-center gap-3 rounded-xl border border-base-200 px-4 py-3 transition hover:bg-base-200/40">
        <input
          type="checkbox"
          className="checkbox checkbox-sm"
          checked={saved}
          onChange={(e) => setSaved(e.target.checked)}
        />
        <span className="text-sm text-base-content/80">{checkboxLabel}</span>
      </label>
      <Btn
        variant="primary"
        icon={ArrowRight}
        disabled={!saved}
        loading={busy}
        onClick={onConfirm}
      >
        {confirmLabel}
      </Btn>
    </>
  );
};

/* --------------------------------------------------------- totp panel --- */

export const TotpSetupPanel: React.FC<{
  otpauthUri: string;
  secret: string;
  code: string;
  onCodeChange: (value: string) => void;
  onVerify: () => void;
  invalid?: boolean;
  busy?: boolean;
  errorMessage?: string | null;
  verifyLabel?: string;
  steps?: { open: string; openSub: string; scan: string; enter: string };
}> = ({
  otpauthUri,
  secret,
  code,
  onCodeChange,
  onVerify,
  invalid,
  busy,
  errorMessage,
  verifyLabel = "Verify & enable",
  steps,
}) => {
  const [reveal, setReveal] = useState(false);
  const copy = {
    open: steps?.open ?? "Open your authenticator app",
    openSub:
      steps?.openSub ?? "Google Authenticator, 1Password, Authy, or similar.",
    scan: steps?.scan ?? "Scan this QR code",
    enter: steps?.enter ?? "Enter the 6-digit code",
  };
  const stepNum = (n: number) => (
    <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-base-200 text-xs font-semibold text-base-content/70">
      {n}
    </span>
  );
  return (
    <>
      <ol className="space-y-3 text-sm">
        <li className="flex gap-3">
          {stepNum(1)}
          <div className="pt-0.5">
            <p className="font-medium text-base-content">{copy.open}</p>
            <p className="text-base-content/55">{copy.openSub}</p>
          </div>
        </li>
        <li className="flex gap-3">
          {stepNum(2)}
          <div className="w-full pt-0.5">
            <p className="font-medium text-base-content">{copy.scan}</p>
            <div className="mt-3 flex flex-col items-center gap-2.5">
              <AuthQR value={otpauthUri} />
              <button
                type="button"
                onClick={() => setReveal((r) => !r)}
                className="text-xs font-medium text-base-content/45 underline-offset-2 hover:underline"
              >
                {reveal ? "Hide setup key" : "Can’t scan? Enter key manually"}
              </button>
              {reveal && (
                <div className="flex items-center gap-1 rounded-lg border border-base-200 bg-base-200/50 py-1 pl-3 pr-1">
                  <code className="text-sm font-medium tracking-wider text-base-content">
                    {secret}
                  </code>
                  <CopyButton text={secret.replace(/ /g, "")} />
                </div>
              )}
            </div>
          </div>
        </li>
        <li className="flex gap-3">
          {stepNum(3)}
          <div className="w-full pt-0.5">
            <p className="font-medium text-base-content">{copy.enter}</p>
            <div className="mt-3">
              <OtpInput
                value={code}
                onChange={onCodeChange}
                onComplete={onVerify}
                invalid={invalid}
                autoFocus={false}
              />
              {invalid && errorMessage && (
                <p className="mt-2 flex items-center gap-1 text-xs font-medium text-error">
                  <CircleAlert size={13} /> {errorMessage}
                </p>
              )}
            </div>
          </div>
        </li>
      </ol>
      <Btn
        variant="primary"
        loading={busy}
        onClick={onVerify}
        disabled={code.length < 6}
      >
        {verifyLabel}
      </Btn>
    </>
  );
};

/* --------------------------------------------------------------- alert --- */

export const InlineError: React.FC<{ children: ReactNode }> = ({
  children,
}) => (
  <div className="flex items-start gap-2 rounded-xl bg-error/10 px-3.5 py-2.5 text-sm text-error">
    <CircleAlert size={15} className="mt-0.5 shrink-0" />
    <span>{children}</span>
  </div>
);

/* ------------------------------------------------------------- success --- */

export const SuccessCard: React.FC<{
  icon: LucideIcon;
  title: string;
  subtitle: string;
  ctaLabel: string;
  onCta: () => void;
  badge?: string;
  appName?: string;
}> = ({
  icon: Icon,
  title,
  subtitle,
  ctaLabel,
  onCta,
  badge = "Two-factor authentication active",
  appName,
}) => (
  <AuthShell appName={appName}>
    <div className="flex flex-col items-center gap-5 py-3 text-center">
      <div className="success-ring grid h-20 w-20 place-items-center rounded-full bg-success/12 text-success">
        <Icon size={38} />
      </div>
      <div>
        <h1 className="text-2xl font-semibold tracking-tight text-base-content">
          {title}
        </h1>
        <p className="mx-auto mt-2 max-w-xs text-sm leading-relaxed text-base-content/55">
          {subtitle}
        </p>
      </div>
      {badge && (
        <div className="flex items-center gap-2 rounded-full bg-base-200 px-3.5 py-1.5 text-xs font-medium text-base-content/60">
          <Lock size={13} /> {badge}
        </div>
      )}
      <Btn
        variant="neutral"
        icon={ArrowRight}
        className="w-full"
        onClick={onCta}
      >
        {ctaLabel}
      </Btn>
    </div>
  </AuthShell>
);

/** Stable id helper for label/input pairing in forms. */
export function useFieldId(prefix: string): string {
  const id = useId();
  return useMemo(() => `${prefix}-${id}`, [prefix, id]);
}
