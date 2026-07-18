import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useBeginTOTPSetup,
  useDisableTOTP,
  useEnableTOTP,
  useMFAStatus,
  useRegenerateRecoveryCodes,
  type TOTPSetupResponse,
} from "../../api/useMFA.ts";

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

export function useMFAFlow() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const statusQuery = useMFAStatus();
  const beginSetupMutation = useBeginTOTPSetup();
  const enableTOTP = useEnableTOTP();
  const disableTOTP = useDisableTOTP();
  const regenerateRecoveryCodes = useRegenerateRecoveryCodes();
  const autoSetupTriggeredRef = useRef(false);

  const [setupResponse, setSetupResponse] = useState<TOTPSetupResponse | null>(null);
  const [verificationCode, setVerificationCode] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [activeAction, setActiveAction] = useState<"disable" | "regenerate" | null>(null);

  const status = statusQuery.data;
  const shouldAutoStartSetup = searchParams.get("mfa") === "setup";
  const requestedAction = searchParams.get("action");
  const backTo = useMemo(() => {
    const from = (location.state as ReturnState | null)?.from;
    if (!from?.pathname) {
      return "/settings?tab=account";
    }
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const clearFlowParams = (...keys: string[]) => {
    const nextParams = new URLSearchParams(searchParams);
    for (const key of keys) {
      nextParams.delete(key);
    }
    setSearchParams(nextParams, { replace: true });
  };

  const resetAction = () => {
    setActiveAction(null);
    setPassword("");
    setError(null);
    clearFlowParams("action");
  };

  const handleBeginSetup = async () => {
    setError(null);
    try {
      const payload = await beginSetupMutation.mutateAsync({});
      if (payload) {
        setSetupResponse(payload);
        setVerificationCode("");
        setRecoveryCodes([]);
        setActiveAction(null);
      }
    } catch (cause) {
      setError(
        getErrorMessage(
          cause,
          t("settings.account.mfa.setupError", {
            defaultValue: "Failed to start TOTP setup.",
          }),
        ),
      );
    }
  };

  const handleEnable = async () => {
    const setupToken = setupResponse?.setup_token;
    if (!setupToken || verificationCode.length < 6) return;
    setError(null);
    try {
      const payload = await enableTOTP.mutateAsync({
        body: {
          setup_token: setupToken,
          code: verificationCode,
        },
      });
      setRecoveryCodes(payload?.recovery_codes ?? []);
      setSetupResponse(null);
      setVerificationCode("");
      clearFlowParams("mfa", "action");
    } catch (cause) {
      setError(
        getErrorMessage(
          cause,
          t("settings.account.mfa.enableError", {
            defaultValue: "Failed to enable TOTP.",
          }),
        ),
      );
    }
  };

  const handleDisable = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    try {
      await disableTOTP.mutateAsync({ body: { current_password: password } });
      resetAction();
      setRecoveryCodes([]);
    } catch (cause) {
      setError(
        getErrorMessage(
          cause,
          t("settings.account.mfa.disableError", {
            defaultValue: "Failed to disable TOTP.",
          }),
        ),
      );
    }
  };

  const handleRegenerate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    try {
      const payload = await regenerateRecoveryCodes.mutateAsync({
        body: { current_password: password },
      });
      setRecoveryCodes(payload?.recovery_codes ?? []);
      setActiveAction(null);
      setPassword("");
      clearFlowParams("action");
    } catch (cause) {
      setError(
        getErrorMessage(
          cause,
          t("settings.account.mfa.regenerateError", {
            defaultValue: "Failed to regenerate recovery codes.",
          }),
        ),
      );
    }
  };

  useEffect(() => {
    if (
      !shouldAutoStartSetup ||
      autoSetupTriggeredRef.current ||
      statusQuery.isLoading ||
      status?.totp_enabled ||
      setupResponse ||
      beginSetupMutation.isPending
    ) {
      return;
    }
    autoSetupTriggeredRef.current = true;
    void handleBeginSetup();
  }, [
    beginSetupMutation.isPending,
    setupResponse,
    shouldAutoStartSetup,
    status?.totp_enabled,
    statusQuery.isLoading,
  ]);

  useEffect(() => {
    if (statusQuery.isLoading || !status?.totp_enabled) return;
    if (requestedAction === "disable") setActiveAction("disable");
    else if (requestedAction === "regenerate") setActiveAction("regenerate");
  }, [requestedAction, status?.totp_enabled, statusQuery.isLoading]);

  return {
    statusQuery,
    status,
    setupResponse,
    verificationCode,
    setVerificationCode,
    password,
    setPassword,
    error,
    recoveryCodes,
    activeAction,
    setActiveAction,
    backTo,
    resetAction,
    handleBeginSetup,
    handleEnable,
    handleDisable,
    handleRegenerate,
    finishRecoveryCodes: () => navigate(backTo),
    isBeginningSetup: beginSetupMutation.isPending,
    isEnabling: enableTOTP.isPending,
    isDisabling: disableTOTP.isPending,
    isRegenerating: regenerateRecoveryCodes.isPending,
  };
}
