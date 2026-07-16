import React, { useEffect, useRef, useState } from "react";
import { Navigate, useNavigate } from "react-router-dom";
import { AlertCircle, KeyRound, ShieldCheck } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "../hooks/useAuth.ts";
import { takeRequiredPasswordChangeChallenge } from "../passwordChangeChallenge.ts";
import {
  PASSWORD_HINT,
  PASSWORD_MAX_LENGTH,
  PASSWORD_MIN_LENGTH,
  PASSWORD_PATTERN,
} from "../lib/credentialPolicy.ts";

function apiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const value = error as { message?: string; error?: string };
    return value.message || value.error || fallback;
  }
  return fallback;
}

export default function RequiredPasswordChangePage(): React.ReactNode {
  const { t } = useI18n();
  const { completeAuth } = useAuth();
  const navigate = useNavigate();
  const [challenge] = useState(takeRequiredPasswordChangeChallenge);
  const mutation = $api.useMutation("post", "/api/v1/auth/password-change/complete");
  const confirmRef = useRef<HTMLInputElement | null>(null);
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);

  const mismatch = t("auth.requiredPasswordChange.mismatch", "Passwords must match exactly.");
  useEffect(() => {
    const input = confirmRef.current;
    if (!input) return;
    input.setCustomValidity(confirmPassword && confirmPassword !== newPassword ? mismatch : "");
  }, [confirmPassword, mismatch, newPassword]);

  if (!challenge?.passwordChangeToken) {
    return <Navigate to="/login" replace />;
  }

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    if (confirmRef.current && !confirmRef.current.checkValidity()) {
      confirmRef.current.reportValidity();
      return;
    }
    try {
      const response = await mutation.mutateAsync({
        body: {
          password_change_token: challenge.passwordChangeToken,
          new_password: newPassword,
        },
      });
      await completeAuth(response);
      void navigate(challenge.redirectTo || "/", { replace: true });
    } catch (cause) {
      setError(
        apiMessage(
          cause,
          t("auth.requiredPasswordChange.error", "Unable to set the new password."),
        ),
      );
    }
  };

  return (
    <div className="flex min-h-dvh items-center justify-center bg-base-200 px-4 py-12">
      <div className="card w-full max-w-lg bg-base-100 shadow-xl">
        <div className="card-body gap-5 p-8 sm:p-10">
          <div className="flex flex-col items-center gap-3 text-center">
            <ShieldCheck className="size-10 text-primary" aria-hidden="true" />
            <div>
              <h1 className="card-title justify-center text-2xl">
                {t("auth.requiredPasswordChange.title", "Choose a new password")}
              </h1>
              <p className="mt-2 text-sm text-base-content/70">
                {t(
                  "auth.requiredPasswordChange.description",
                  "Your temporary password was accepted. Set a permanent password before continuing.",
                )}
              </p>
              {challenge.username && (
                <p className="mt-1 text-sm font-medium text-base-content">{challenge.username}</p>
              )}
            </div>
          </div>

          {error && (
            <div role="alert" className="alert alert-error">
              <AlertCircle className="size-4" aria-hidden="true" />
              <span>{error}</span>
            </div>
          )}

          <form className="flex flex-col gap-4" onSubmit={submit}>
            <fieldset className="fieldset">
              <legend className="fieldset-legend">
                {t("auth.requiredPasswordChange.newPassword", "New password")}
              </legend>
              <label className="input flex w-full items-center gap-2">
                <KeyRound className="size-4 text-base-content/60" aria-hidden="true" />
                <input
                  type="password"
                  aria-label={t("auth.requiredPasswordChange.newPassword", "New password")}
                  className="grow"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  minLength={PASSWORD_MIN_LENGTH}
                  maxLength={PASSWORD_MAX_LENGTH}
                  pattern={PASSWORD_PATTERN}
                  autoComplete="new-password"
                  required
                />
              </label>
              <p className="label">
                {t("auth.requiredPasswordChange.passwordHint", PASSWORD_HINT)}
              </p>
            </fieldset>

            <fieldset className="fieldset">
              <legend className="fieldset-legend">
                {t("auth.requiredPasswordChange.confirmPassword", "Confirm new password")}
              </legend>
              <label className="input flex w-full items-center gap-2">
                <KeyRound className="size-4 text-base-content/60" aria-hidden="true" />
                <input
                  ref={confirmRef}
                  type="password"
                  aria-label={t(
                    "auth.requiredPasswordChange.confirmPassword",
                    "Confirm new password",
                  )}
                  className="grow"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  autoComplete="new-password"
                  required
                />
              </label>
            </fieldset>

            <button
              type="submit"
              className="btn btn-primary btn-block"
              disabled={mutation.isPending}
            >
              {mutation.isPending
                ? t("auth.requiredPasswordChange.saving", "Saving…")
                : t("auth.requiredPasswordChange.submit", "Set password and continue")}
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
