import React, { createContext, useContext, useId, useState, type ReactNode } from "react";
import { CircleAlert, Eye, EyeOff, Info, Lock, type LucideIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { cx } from "./classNames.ts";
import { passwordStrength } from "./passwordStrength.ts";

/** Carries the generated id from {@link Field} down to {@link FieldInput}. */
const FieldIdContext = createContext<string | undefined>(undefined);

/**
 * Input that adopts the enclosing {@link Field}'s id, so the label/input pairing
 * that gives the control its accessible name cannot be forgotten at a call site.
 */
const FieldInput = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  function FieldInput({ id, ...props }, ref) {
    const fieldId = useContext(FieldIdContext);
    return <input ref={ref} id={id ?? fieldId} {...props} />;
  },
);

export const Field: React.FC<{
  label?: ReactNode;
  hint?: string;
  error?: ReactNode;
  children: ReactNode;
}> = ({ label, hint, error, children }) => {
  const id = useId();
  return (
    <div className="form-control w-full min-w-0">
      {(label || hint) && (
        <div className="mb-1.5 flex min-w-0 items-center gap-1.5">
          {label && (
            <label className="min-w-0 text-sm font-medium text-base-content/80" htmlFor={id}>
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
      <FieldIdContext.Provider value={id}>{children}</FieldIdContext.Provider>
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

export const TextInput = React.forwardRef<HTMLInputElement, TextInputProps>(function TextInput(
  { icon: Icon, invalid, className, ...props },
  ref,
) {
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
        <FieldInput
          ref={ref}
          className="min-w-0 grow bg-transparent outline-none placeholder:text-base-content/35"
          {...props}
        />
      </div>
    );
  }
  return (
    <FieldInput
      ref={ref}
      className={cx(
        "input input-bordered w-full min-w-0 bg-base-100",
        invalid && "input-error",
        className,
      )}
      {...props}
    />
  );
});

const STRENGTH_COLORS = ["bg-base-300", "bg-error", "bg-warning", "bg-success/70", "bg-success"];

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
};

export const PasswordField: React.FC<PasswordFieldProps> = ({
  label,
  hint,
  error,
  value,
  onChange,
  placeholder = "••••••••",
  autoComplete = "new-password",
  meter = false,
  inputRef,
}) => {
  const { t } = useI18n();
  const [show, setShow] = useState(false);
  const score = passwordStrength(value);
  const strengthLabels = [
    t("auth.passwordStrength.tooShort", "Too short"),
    t("auth.passwordStrength.weak", "Weak"),
    t("auth.passwordStrength.fair", "Fair"),
    t("auth.passwordStrength.good", "Good"),
    t("auth.passwordStrength.strong", "Strong"),
  ];
  return (
    <Field label={label ?? t("auth.passwordField.password", "Password")} hint={hint} error={error}>
      <div
        className={cx(
          "input input-bordered flex w-full min-w-0 items-center gap-2.5 bg-base-100",
          !!error && "input-error",
        )}
      >
        <Lock size={17} className="shrink-0 text-base-content/40" />
        <FieldInput
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
          aria-label={
            show
              ? t("auth.passwordField.hidePassword", "Hide password")
              : t("auth.passwordField.showPassword", "Show password")
          }
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
                  i <= score ? STRENGTH_COLORS[score] : "bg-base-300",
                )}
              />
            ))}
          </div>
          <span className="w-12 text-right text-xs font-medium text-base-content/50">
            {strengthLabels[score]}
          </span>
        </div>
      )}
    </Field>
  );
};
