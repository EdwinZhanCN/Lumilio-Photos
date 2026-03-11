import React, { useEffect, useMemo, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  AlertCircle,
  ArrowLeft,
  Info,
  KeyRound,
  ShieldCheck,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "../hooks/useAuth.ts";
import { useChangeMyPassword } from "@/features/users/hooks/useUsers";
import {
  PASSWORD_HINT,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  PASSWORD_PATTERN,
} from "../lib/credentialPolicy.ts";

type ReturnState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const maybeApiError = error as { message?: string; error?: string };
    if (maybeApiError.message) return maybeApiError.message;
    if (maybeApiError.error) return maybeApiError.error;
  }
  return fallback;
}

export default function ChangePasswordPage(): React.ReactNode {
  const { t } = useI18n();
  const { logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const changePasswordMutation = useChangeMyPassword();
  const confirmPasswordRef = useRef<HTMLInputElement | null>(null);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const backTo = useMemo(() => {
    const from = (location.state as ReturnState | null)?.from;
    if (!from?.pathname) {
      return "/settings?tab=account";
    }
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const confirmPasswordMessage = t("auth.changePassword.mismatch", {
    defaultValue: "Passwords must match exactly.",
  });

  const isBusy = changePasswordMutation.isPending;

  useEffect(() => {
    const input = confirmPasswordRef.current;
    if (!input) return;

    if (confirmPassword && confirmPassword !== newPassword) {
      input.setCustomValidity(confirmPasswordMessage);
      return;
    }

    input.setCustomValidity("");
  }, [confirmPassword, newPassword, confirmPasswordMessage]);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMessage(null);

    if (
      confirmPasswordRef.current &&
      !confirmPasswordRef.current.checkValidity()
    ) {
      confirmPasswordRef.current.reportValidity();
      return;
    }

    try {
      await changePasswordMutation.mutateAsync({
        body: {
          current_password: currentPassword,
          new_password: newPassword,
        },
      });

      logout();
      navigate("/login", { replace: true });
    } catch (error) {
      setErrorMessage(
        getErrorMessage(
          error,
          t("auth.changePassword.error", {
            defaultValue: "Failed to change password.",
          }),
        ),
      );
    }
  };

  return (
    <div className="relative flex min-h-screen w-full items-center justify-center overflow-hidden bg-base-200 px-4 py-12">
      <div className="pointer-events-none absolute inset-0 overflow-hidden">
        <div className="absolute -right-48 -top-48 h-96 w-96 rounded-full bg-primary/8 blur-3xl" />
        <div className="absolute -bottom-48 -left-48 h-96 w-96 rounded-full bg-secondary/8 blur-3xl" />
      </div>

      <div className="relative w-full max-w-lg">
        <div className="card bg-base-100 shadow-2xl ring-1 ring-base-content/5">
          <div className="card-body gap-4 p-8 sm:p-10">
            {/* Header */}
            <div className="flex flex-col items-center gap-5 text-center">
              <div className="inline-flex items-center gap-4 text-3xl font-bold tracking-tight text-base-content sm:text-4xl">
                <img
                  src="/logo.png"
                  alt={t("app.name") + " Logo"}
                  className="size-10 bg-contain object-contain sm:size-12"
                />
                <span>{t("app.name")}</span>
              </div>

              <div className="space-y-1.5">
                <h1 className="text-xl font-semibold tracking-tight sm:text-2xl">
                  {t("auth.changePassword.pageTitle", {
                    defaultValue: "Change your password",
                  })}
                </h1>

                <p className="text-sm text-base-content/80">
                  {t("auth.changePassword.pageSubtitle", {
                    defaultValue:
                      "After changing your password, you will be signed out and need to log in again with the new password.",
                  })}
                </p>
              </div>
            </div>

            {/* Error alert */}
            {errorMessage && (
              <div className="alert alert-error py-3 text-sm">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{errorMessage}</span>
              </div>
            )}

            {/* Form */}
            <form className="flex flex-col gap-2.5" onSubmit={handleSubmit}>
              {/* Current password */}
              <fieldset className="fieldset">
                <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                  {t("auth.changePassword.currentPassword", {
                    defaultValue: "Current password",
                  })}
                </legend>
                <label className="input input-bordered validator flex w-full items-center gap-2">
                  <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                  <input
                    type="password"
                    placeholder="••••••••"
                    className="grow"
                    value={currentPassword}
                    onChange={(event) => setCurrentPassword(event.target.value)}
                    autoComplete="current-password"
                    required
                  />
                  <div
                    className="tooltip tooltip-left cursor-help"
                    data-tip={t("auth.changePassword.currentPasswordHint", {
                      defaultValue:
                        "Enter your existing password to verify your identity.",
                    })}
                  >
                    <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                  </div>
                </label>
              </fieldset>

              <div className="divider my-1 text-xs text-base-content/50">
                {t("auth.changePassword.newPasswordDivider", {
                  defaultValue: "NEW PASSWORD",
                })}
              </div>

              {/* New password */}
              <fieldset className="fieldset">
                <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                  {t("auth.changePassword.newPassword", {
                    defaultValue: "New password",
                  })}
                </legend>
                <label className="input input-bordered validator flex w-full items-center gap-2">
                  <KeyRound className="h-4 w-4 shrink-0 text-base-content/70" />
                  <input
                    type="password"
                    placeholder="••••••••"
                    className="grow"
                    value={newPassword}
                    onChange={(event) => setNewPassword(event.target.value)}
                    autoComplete="new-password"
                    minLength={PASSWORD_MIN_LENGTH}
                    maxLength={PASSWORD_MAX_LENGTH}
                    pattern={PASSWORD_PATTERN}
                    required
                  />
                  <div
                    className="tooltip tooltip-left cursor-help"
                    data-tip={t("auth.changePassword.passwordHint", {
                      defaultValue: PASSWORD_HINT,
                    })}
                  >
                    <Info className="h-3.5 w-3.5 text-base-content/70 transition-colors hover:text-base-content" />
                  </div>
                </label>
              </fieldset>

              {/* Confirm password */}
              <fieldset className="fieldset">
                <legend className="fieldset-legend text-xs font-medium uppercase tracking-wider text-base-content/70">
                  {t("auth.changePassword.confirmPassword", {
                    defaultValue: "Confirm new password",
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
                {!isBusy && <KeyRound className="h-4 w-4" />}
                {isBusy
                  ? t("auth.changePassword.loading", {
                      defaultValue: "Updating password…",
                    })
                  : t("auth.changePassword.submit", {
                      defaultValue: "Update password",
                    })}
              </button>
            </form>

            <div className="divider my-0 text-xs text-base-content/70">
              {t("common.or", { defaultValue: "OR" })}
            </div>

            <div className="text-center">
              <p className="text-xs text-base-content/70">
                {t("auth.changePassword.changed", {
                  defaultValue: "Changed your mind?",
                })}
              </p>
              <Link to={backTo} className="btn btn-link btn-sm mt-1 gap-1.5">
                <ArrowLeft className="h-3.5 w-3.5" />
                {t("auth.changePassword.backToSettings", {
                  defaultValue: "Back to settings",
                })}
              </Link>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
