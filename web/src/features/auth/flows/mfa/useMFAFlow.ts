import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import { localizeProblem } from "@/lib/http-commons/problem";
import { useAuth } from "../../state/useAuth.ts";
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
  const [activeAction, setActiveAction] = useState<"setup" | "disable" | "regenerate" | null>(null);
  const [securityCode, setSecurityCode] = useState("");
  const [securityMethod, setSecurityMethod] = useState<"totp" | "recovery_code">("totp");
  const { completeAuth } = useAuth();
  const securityVerify = $api.useMutation("post", "/api/v1/auth/security/verify");

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
    setSecurityCode("");
    setSecurityMethod("totp");
    setError(null);
    clearFlowParams("action");
  };

  const handleBeginSetup = async () => {
    setError(null);
    try {
      const security = await securityVerify.mutateAsync({
        body: {
          current_password: password,
          ...(status?.totp_enabled ? { code: securityCode, method: securityMethod } : {}),
          purpose: "totp_setup",
        },
      });
      if (!security?.security_token) {
        throw new Error(
          t("settings.account.mfa.securityVerificationError", {
            defaultValue: "Unable to verify account security.",
          }),
        );
      }
      const payload = await beginSetupMutation.mutateAsync({
        body: { security_token: security.security_token },
      });
      if (payload) {
        setSetupResponse(payload);
        setVerificationCode("");
        setSecurityCode("");
        setRecoveryCodes([]);
        setActiveAction(null);
      }
    } catch (cause) {
      setError(
        localizeProblem(
          cause,
          t,
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
      if (payload?.status?.totp_enabled !== true || !payload.session) {
        throw new Error(
          t("settings.account.mfa.enableError", {
            defaultValue: "TOTP activation was not confirmed by the server.",
          }),
        );
      }
      const codes = payload.recovery_codes ?? [];
      if (codes.length === 0) {
        throw new Error(
          t("settings.account.mfa.enableError", {
            defaultValue: "TOTP activation did not return recovery codes.",
          }),
        );
      }
      await completeAuth(payload.session);
      setRecoveryCodes(codes);
      setSetupResponse(null);
      setVerificationCode("");
      clearFlowParams("mfa", "action");
    } catch (cause) {
      setError(
        localizeProblem(
          cause,
          t,
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
      const security = await securityVerify.mutateAsync({
        body: {
          current_password: password,
          code: securityCode,
          method: securityMethod,
          purpose: "totp_disable",
        },
      });
      if (!security?.security_token) throw new Error("Invalid security verification");
      const response = await disableTOTP.mutateAsync({
        body: { security_token: security.security_token },
      });
      if (!response?.session) throw new Error("Session replacement was not returned");
      await completeAuth(response.session);
      resetAction();
      setRecoveryCodes([]);
    } catch (cause) {
      setError(
        localizeProblem(
          cause,
          t,
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
      const security = await securityVerify.mutateAsync({
        body: {
          current_password: password,
          code: securityCode,
          method: securityMethod,
          purpose: "recovery_regenerate",
        },
      });
      if (!security?.security_token) throw new Error("Invalid security verification");
      const payload = await regenerateRecoveryCodes.mutateAsync({
        body: { security_token: security.security_token },
      });
      if (!payload?.session) throw new Error("Session replacement was not returned");
      await completeAuth(payload.session);
      setRecoveryCodes(payload?.recovery_codes ?? []);
      setActiveAction(null);
      setPassword("");
      clearFlowParams("action");
    } catch (cause) {
      setError(
        localizeProblem(
          cause,
          t,
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
      activeAction ||
      beginSetupMutation.isPending
    ) {
      return;
    }
    autoSetupTriggeredRef.current = true;
    setActiveAction("setup");
  }, [
    beginSetupMutation.isPending,
    setupResponse,
    shouldAutoStartSetup,
    status?.totp_enabled,
    statusQuery.isLoading,
    activeAction,
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
    securityCode,
    setSecurityCode,
    securityMethod,
    setSecurityMethod,
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
    isBeginningSetup: beginSetupMutation.isPending || securityVerify.isPending,
    isEnabling: enableTOTP.isPending,
    isDisabling: disableTOTP.isPending,
    isRegenerating: regenerateRecoveryCodes.isPending,
  };
}
