import React from "react";
import { Navigate, useLocation } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useBootstrapStatus } from "../hooks/useBootstrapStatus.ts";

const BOOTSTRAP_PATH = "/bootstrap";

const isBootstrapPath = (pathname: string) =>
  pathname === BOOTSTRAP_PATH || pathname.startsWith(`${BOOTSTRAP_PATH}/`);

const BootstrapGate: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { t } = useI18n();
  const location = useLocation();
  const bootstrapQuery = useBootstrapStatus();
  const isBootstrapMode = bootstrapQuery.data?.data?.is_bootstrap_mode ?? false;
  const isBootstrapRoute = isBootstrapPath(location.pathname);

  if (bootstrapQuery.isLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-base-200">
        <div className="flex flex-col items-center gap-4">
          <span className="loading loading-spinner loading-lg text-primary"></span>
          <p className="text-sm font-medium opacity-50 animate-pulse">
            {t("auth.bootstrap.gate.loading", {
              defaultValue: "Preparing setup...",
            })}
          </p>
        </div>
      </div>
    );
  }

  if (bootstrapQuery.isError) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-base-200 px-4">
        <div className="alert alert-error max-w-md text-sm">
          <div className="flex flex-col gap-3">
            <span>
              {t("auth.bootstrap.gate.statusError", {
                defaultValue: "Unable to verify bootstrap status.",
              })}
            </span>
            <button
              type="button"
              className="btn btn-sm btn-primary"
              onClick={() => void bootstrapQuery.refetch()}
            >
              {t("common.retry", { defaultValue: "Retry" })}
            </button>
          </div>
        </div>
      </div>
    );
  }

  if (isBootstrapMode && !isBootstrapRoute) {
    return <Navigate to={BOOTSTRAP_PATH} replace />;
  }

  if (!isBootstrapMode && isBootstrapRoute) {
    return <Navigate to="/login" replace />;
  }

  return <>{children}</>;
};

export default BootstrapGate;
