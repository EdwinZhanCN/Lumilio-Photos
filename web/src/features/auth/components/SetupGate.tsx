import React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { HardDrive, KeyRound, ShieldCheck } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { setupStatusQueryKey, useSetupStatus } from "../hooks/useSetupStatus.ts";

function getApiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const apiError = error as { message?: string; error?: string };
    if (apiError.message) return apiError.message;
    if (apiError.error) return apiError.error;
  }
  return fallback;
}

/**
 * First-boot STEP ①: rotate the temporary bootstrap database credential to a
 * high-entropy secret and generate the app key. This is a one-time, user-
 * initiated action (a welcome screen with an explicit button) — it is not run
 * automatically. Once the credential is rotated the admin/MFA wizard
 * (BootstrapGate) and the primary-repository step take over.
 */
const SetupGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const setupMutation = $api.useMutation("post", "/api/v1/setup");
  const databaseInitialized =
    setupQuery.data?.database_initialized ?? setupQuery.data?.initialized ?? false;

  const beginSetup = () => {
    void setupMutation
      .mutateAsync({ body: {} })
      .then(() => queryClient.invalidateQueries({ queryKey: setupStatusQueryKey }))
      .catch(() => undefined);
  };

  if (setupQuery.isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-200">
        <div className="flex flex-col items-center gap-4">
          <span className="loading loading-spinner loading-lg text-primary" />
          <p className="animate-pulse text-sm font-medium opacity-50">
            {t("auth.setup.gate.loading", {
              defaultValue: "Checking system status…",
            })}
          </p>
        </div>
      </div>
    );
  }

  if (setupQuery.isError) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-200 px-4">
        <div className="alert alert-error max-w-md text-sm">
          <div className="flex flex-col gap-3">
            <span>
              {t("auth.setup.gate.statusError", {
                defaultValue: "Unable to verify system status.",
              })}
            </span>
            <button
              type="button"
              className="btn btn-sm btn-primary"
              onClick={() => void setupQuery.refetch()}
            >
              {t("common.retry", { defaultValue: "Retry" })}
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (!databaseInitialized) {
    const setupError = setupMutation.isError
      ? getApiMessage(
          setupMutation.error,
          t("auth.setup.error", {
            defaultValue: "Setup failed. Check the server logs and try again.",
          }),
        )
      : null;

    return (
      <div className="grid min-h-dvh place-items-center bg-base-200 px-4 py-10">
        <div className="w-full max-w-md rounded-2xl border border-base-200 bg-base-100 p-7 shadow-[0_1px_2px_rgba(0,0,0,0.04),0_18px_44px_-18px_rgba(0,0,0,0.18)] sm:p-9">
          <div className="grid h-12 w-12 place-items-center rounded-xl bg-base-200 text-base-content">
            <ShieldCheck size={24} />
          </div>
          <h1 className="mt-5 text-2xl font-semibold tracking-tight">
            {t("auth.setup.welcome.title", {
              defaultValue: "Initialize this server",
            })}
          </h1>
          <p className="mt-2 text-sm leading-relaxed text-base-content/55">
            {t("auth.setup.welcome.body", {
              defaultValue:
                "Before the first account is created, Lumilio secures the database and generates an application key. This runs once and takes a few seconds.",
            })}
          </p>

          <dl className="mt-5 grid gap-2.5">
            <div className="flex items-start gap-3 rounded-xl border border-base-200 px-4 py-3">
              <KeyRound size={18} className="mt-0.5 shrink-0 text-base-content/45" />
              <div>
                <p className="text-sm font-medium">
                  {t("auth.setup.welcome.rotateTitle", {
                    defaultValue: "Rotate the database credential",
                  })}
                </p>
                <p className="text-xs text-base-content/55">
                  {t("auth.setup.welcome.rotateBody", {
                    defaultValue:
                      "Replaces the temporary bootstrap password with a high-entropy secret stored under your storage root.",
                  })}
                </p>
              </div>
            </div>
            <div className="flex items-start gap-3 rounded-xl border border-base-200 px-4 py-3">
              <HardDrive size={18} className="mt-0.5 shrink-0 text-base-content/45" />
              <div>
                <p className="text-sm font-medium">
                  {t("auth.setup.welcome.keyTitle", {
                    defaultValue: "Generate the application key",
                  })}
                </p>
                <p className="text-xs text-base-content/55">
                  {t("auth.setup.welcome.keyBody", {
                    defaultValue:
                      "Used to sign sessions and encrypt secrets; kept in .secrets and never leaves this server.",
                  })}
                </p>
              </div>
            </div>
          </dl>

          {setupError && (
            <div className="alert alert-error mt-5 text-left text-sm">
              <span>{setupError}</span>
            </div>
          )}

          <button
            type="button"
            className="btn btn-neutral mt-6 w-full"
            onClick={beginSetup}
            disabled={setupMutation.isPending}
          >
            {setupMutation.isPending ? (
              <>
                <span className="loading loading-spinner loading-sm" />
                {t("auth.setup.welcome.working", {
                  defaultValue: "Securing database credentials…",
                })}
              </>
            ) : setupMutation.isError ? (
              t("common.retry", { defaultValue: "Retry" })
            ) : (
              t("auth.setup.welcome.cta", {
                defaultValue: "Begin initialization",
              })
            )}
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

export default SetupGate;
