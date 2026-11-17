import React from "react";
import { AlertTriangle, HardDrive, RefreshCw } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSetupStatus } from "../../api/useSetupStatus.ts";
import PrimarySetupPanel from "../../flows/primary-setup/PrimarySetupPanel.tsx";

/**
 * Keeps the app behind the Primary Repository until setup created or adopted
 * one. A degraded, established instance stays usable with a recovery banner:
 * setup completion is durable and never reopened by a runtime storage fault.
 */
const PrimaryRepositoryGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const setupQuery = useSetupStatus();
  const primaryReady = setupQuery.data?.primary_repository_initialized ?? false;

  if (setupQuery.isLoading) {
    return (
      <div className="flex min-h-dvh items-center justify-center bg-base-200">
        <div className="flex flex-col items-center gap-4">
          <span className="loading loading-spinner loading-lg text-primary" />
          <p className="animate-pulse text-sm font-medium opacity-50">
            {t("auth.primaryRepository.loading", {
              defaultValue: "Checking Primary Repository setup...",
            })}
          </p>
        </div>
      </div>
    );
  }

  if (primaryReady) {
    if (setupQuery.data?.runtime_state === "degraded") {
      return (
        <>
          <div
            role="alert"
            className="alert alert-warning rounded-none border-x-0 border-t-0 px-4 py-3"
          >
            <AlertTriangle className="size-5 shrink-0" />
            <div className="min-w-0 flex-1">
              <strong>{t("auth.storageRecovery.title", "Storage recovery is required")}</strong>
              <p className="mt-0.5 text-sm">
                {t(
                  "auth.storageRecovery.description",
                  "Restore the configured Default Storage Location and its Primary Repository without changing either marker identity. Other available Repositories remain usable.",
                )}
              </p>
            </div>
            <button
              type="button"
              className="btn btn-sm btn-ghost gap-2"
              onClick={() => void setupQuery.refetch()}
              disabled={setupQuery.isFetching}
            >
              <RefreshCw className={`size-4 ${setupQuery.isFetching ? "animate-spin" : ""}`} />
              {t("auth.storageRecovery.checkAgain", "Check Again")}
            </button>
          </div>
          {children}
        </>
      );
    }
    return <>{children}</>;
  }

  return (
    <div className="flex min-h-dvh items-center justify-center bg-base-200 px-4">
      <div className="w-full max-w-xl rounded-lg border border-base-300 bg-base-100 p-6 shadow-xl">
        <div className="mb-6 flex items-start gap-4">
          <div className="flex size-11 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <HardDrive size={22} />
          </div>
          <div>
            <h1 className="text-xl font-semibold">
              {t("auth.primarySetup.title", "Set up the Primary Repository")}
            </h1>
            <p className="mt-1 text-sm text-base-content/70">
              {t("auth.primaryRepository.description", {
                defaultValue:
                  "The Primary Repository is the unique Repository in the Default Storage Location's primary/ folder. Lumilio uses it when no other Repository is selected.",
              })}
            </p>
          </div>
        </div>
        {setupQuery.isError ? (
          <div className="alert alert-error mb-4 text-sm">
            {t("auth.primaryRepository.statusError", {
              defaultValue: "Unable to load Primary Repository defaults.",
            })}
          </div>
        ) : null}
        <PrimarySetupPanel variant="gate" onCompleted={() => void setupQuery.refetch()} />
      </div>
    </div>
  );
};

export default PrimaryRepositoryGate;
