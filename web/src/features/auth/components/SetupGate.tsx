import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSetupStatus } from "../hooks/useSetupStatus.ts";
import SetupPage from "../routes/SetupPage.tsx";

/**
 * Routes all traffic to the first-run setup interface until the system
 * configuration payload exists on disk. Once initialized, the regular app
 * (including the administrator bootstrap flow) renders.
 */
const SetupGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const setupQuery = useSetupStatus();
  const initialized = setupQuery.data?.data?.initialized ?? false;

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
    return <SetupPage />;
  }

  return <>{children}</>;
};

export default SetupGate;
