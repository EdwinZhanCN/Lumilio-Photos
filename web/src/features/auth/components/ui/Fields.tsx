import React, { useId, useMemo, useState, type ReactNode } from "react";
import { CircleAlert, Eye, EyeOff, Info, Lock, type LucideIcon } from "lucide-react";
import { cx } from "./classNames.ts";
import { passwordStrength } from "./passwordStrength.ts";

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
            <label className="min-w-0 text-sm font-medium text-base-content/80" htmlFor={htmlFor}>
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
  const label_ = strengthLabels && st.score > 0 ? strengthLabels[st.score] : st.label;
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
          <span className="w-12 text-right text-xs font-medium text-base-content/50">{label_}</span>
        </div>
      )}
    </Field>
  );
};

/** Stable id helper for label/input pairing in forms. */
export function useFieldId(prefix: string): string {
  const id = useId();
  return useMemo(() => `${prefix}-${id}`, [prefix, id]);
}
