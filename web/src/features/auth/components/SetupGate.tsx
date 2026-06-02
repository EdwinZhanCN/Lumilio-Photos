import React, { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
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
 * Runs first-boot database credential rotation before the administrator
 * bootstrap flow or regular app renders.
 */
const SetupGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const setupMutation = $api.useMutation("post", "/api/v1/setup");
  const requestedSetup = useRef(false);
  const initialized = setupQuery.data?.data?.initialized ?? false;

  useEffect(() => {
    if (setupQuery.isLoading || setupQuery.isError || initialized || requestedSetup.current) {
      return;
    }

    requestedSetup.current = true;
    void setupMutation
      .mutateAsync({ body: {} })
      .catch(() => undefined)
      .finally(() => {
        void queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
      });
  }, [initialized, queryClient, setupMutation, setupQuery.isError, setupQuery.isLoading]);

  if (setupQuery.isLoading) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-base-200">
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
      <div className="flex min-h-screen items-center justify-center bg-base-200 px-4">
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

  if (!initialized) {
    const setupError = setupMutation.error
      ? getApiMessage(
          setupMutation.error,
          t("auth.setup.error", {
            defaultValue: "Setup failed. Check the server logs and try again.",
          }),
        )
      : null;

    return (
      <div className="flex min-h-screen items-center justify-center bg-base-200 px-4">
        <div className="flex w-full max-w-md flex-col items-center gap-4 text-center">
          {setupMutation.isError ? (
            <div className="alert alert-error text-left text-sm">
              <div className="flex flex-col gap-3">
                <span>{setupError}</span>
                <button
                  type="button"
                  className="btn btn-sm btn-primary self-start"
                  onClick={() => {
                    requestedSetup.current = false;
                    setupMutation.reset();
                    void setupQuery.refetch();
                  }}
                >
                  {t("common.retry", { defaultValue: "Retry" })}
                </button>
              </div>
            </div>
          ) : (
            <>
              <span className="loading loading-spinner loading-lg text-primary" />
              <p className="animate-pulse text-sm font-medium opacity-50">
                {t("auth.setup.gate.initializing", {
                  defaultValue: "Securing database credentials...",
                })}
              </p>
            </>
          )}
        </div>
      </div>
    );
  }

  return <>{children}</>;
};

export default SetupGate;
