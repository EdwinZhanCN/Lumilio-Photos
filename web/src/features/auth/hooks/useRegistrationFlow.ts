import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type FormEvent,
  type RefObject,
} from "react";
import { useLocation, useNavigate } from "react-router-dom";
import QRCode from "qrcode";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "./useAuth.ts";
import { bootstrapStatusQueryKey } from "./useBootstrapStatus.ts";
import type {
  ApiResult,
  AuthResponse,
  PasskeyOptionsResponse,
  RegistrationStartResponse,
  RegistrationTOTPCompleteResponse,
  RegistrationTOTPSetupResponse,
} from "../auth.type.ts";
import {
  createPasskeyCredential,
  getPasskeySupport,
} from "../lib/webauthn.ts";

type AuthRedirectState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

export type RegistrationFlowStep = "credentials" | "choose" | "totp" | "recovery";

type RegistrationFlow = {
  sessionId: string;
};

type RegistrationFlowState = {
  step: RegistrationFlowStep;
  username: string;
  setUsername: (value: string) => void;
  password: string;
  setPassword: (value: string) => void;
  confirmPassword: string;
  setConfirmPassword: (value: string) => void;
  confirmPasswordRef: RefObject<HTMLInputElement | null>;
  confirmPasswordMessage: string;
  capabilityMessage: string | null;
  totpSetup: RegistrationTOTPSetupResponse | null;
  totpCode: string;
  setTotpCode: (value: string) => void;
  recoveryCodes: string[];
  qrCodeDataURL: string | null;
  displayError: string | null;
  isBusy: boolean;
  handleStartRegistration: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  handleCreatePasskey: () => Promise<void>;
  handleUseAuthenticatorApp: () => Promise<void>;
  handleCompleteTotp: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  handleFinish: () => void;
};

function getApiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (error && typeof error === "object") {
    const apiError = error as { message?: string; error?: string };
    if (apiError.message) return apiError.message;
    if (apiError.error) return apiError.error;
  }
  return fallback;
}

export function useRegistrationFlow(): RegistrationFlowState {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { completeAuth, isAuthenticated, isLoading, error: authError } = useAuth();
  const startRegistrationMutation = $api.useMutation(
    "post",
    "/api/v1/auth/register/start",
  );
  const passkeyOptionsMutation = $api.useMutation(
    "post",
    "/api/v1/auth/passkeys/register/options",
  );
  const passkeyVerifyMutation = $api.useMutation(
    "post",
    "/api/v1/auth/passkeys/register/verify",
  );
  const totpSetupMutation = $api.useMutation(
    "post",
    "/api/v1/auth/register/totp/setup",
  );
  const totpCompleteMutation = $api.useMutation(
    "post",
    "/api/v1/auth/register/totp/complete",
  );
  const location = useLocation();
  const navigate = useNavigate();
  const confirmPasswordRef = useRef<HTMLInputElement | null>(null);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [step, setStep] = useState<RegistrationFlowStep>("credentials");
  const [flow, setFlow] = useState<RegistrationFlow | null>(null);
  const [flowError, setFlowError] = useState<string | null>(null);
  const [capabilityMessage, setCapabilityMessage] = useState<string | null>(null);
  const [totpSetup, setTotpSetup] = useState<RegistrationTOTPSetupResponse | null>(
    null,
  );
  const [totpCode, setTotpCode] = useState("");
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);
  const [qrCodeDataURL, setQrCodeDataURL] = useState<string | null>(null);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const passkeySupportReason = passkeySupport.reasonKey
    ? t(passkeySupport.reasonKey)
    : null;
  const confirmPasswordMessage = t("auth.register.confirmPasswordHint", {
    defaultValue: "Passwords must match exactly.",
  });
  const displayError = flowError ?? authError;
  const isBusy =
    isLoading ||
    startRegistrationMutation.isPending ||
    passkeyOptionsMutation.isPending ||
    passkeyVerifyMutation.isPending ||
    totpSetupMutation.isPending ||
    totpCompleteMutation.isPending;

  useEffect(() => {
    const input = confirmPasswordRef.current;
    if (!input) return;

    if (confirmPassword && confirmPassword !== password) {
      input.setCustomValidity(confirmPasswordMessage);
      return;
    }

    input.setCustomValidity("");
  }, [confirmPassword, confirmPasswordMessage, password]);

  useEffect(() => {
    if (!isLoading && isAuthenticated && step !== "recovery") {
      navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, redirectTo, step]);

  useEffect(() => {
    let cancelled = false;

    const otpauthURI = totpSetup?.otpauth_uri;
    if (!otpauthURI) {
      setQrCodeDataURL(null);
      return undefined;
    }

    QRCode.toDataURL(otpauthURI, {
      errorCorrectionLevel: "M",
      margin: 1,
      width: 224,
      color: {
        dark: "#111827",
        light: "#ffffff",
      },
    })
      .then((dataURL: string) => {
        if (!cancelled) {
          setQrCodeDataURL(dataURL);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setQrCodeDataURL(null);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [totpSetup?.otpauth_uri]);

  const startTotpSetup = async (sessionId: string) => {
    const setupResponse = await totpSetupMutation.mutateAsync({
      body: {
        registration_session_id: sessionId,
      },
    });
    const payload =
      setupResponse as ApiResult<RegistrationTOTPSetupResponse> | undefined;
    if (!payload?.data) {
      throw new Error(payload?.message || t("auth.register.totpSetupStartError"));
    }

    setTotpSetup(payload.data);
    setTotpCode("");
    setStep("totp");
  };

  const handleStartRegistration = async (
    event: FormEvent<HTMLFormElement>,
  ) => {
    event.preventDefault();
    setFlowError(null);

    if (
      confirmPasswordRef.current &&
      !confirmPasswordRef.current.checkValidity()
    ) {
      confirmPasswordRef.current.reportValidity();
      return;
    }

    try {
      const response = await startRegistrationMutation.mutateAsync({
        body: {
          username,
          password,
        },
      });
      const payload =
        response as ApiResult<RegistrationStartResponse> | undefined;
      if (!payload?.data) {
        throw new Error(payload?.message || t("auth.register.startError"));
      }

      const nextFlow = {
        sessionId: payload.data.registration_session_id ?? "",
      };
      setFlow(nextFlow);

      if (passkeySupport.supported) {
        setCapabilityMessage(null);
        setStep("choose");
        return;
      }

      setCapabilityMessage(
        passkeySupportReason || t("auth.register.passkeyUnavailableUseTotp"),
      );
      await startTotpSetup(nextFlow.sessionId);
    } catch (registrationError) {
      setFlowError(getApiMessage(registrationError, t("auth.register.startError")));
    }
  };

  const handleCreatePasskey = async () => {
    if (!flow) return;
    setFlowError(null);

    try {
      const optionsResponse = await passkeyOptionsMutation.mutateAsync({
        body: {
          registration_session_id: flow.sessionId,
        },
      });
      const optionsData =
        optionsResponse as ApiResult<PasskeyOptionsResponse> | undefined;
      if (!optionsData?.data) {
        throw new Error(
          optionsData?.message || t("auth.register.passkeyStartError"),
        );
      }

      const credential = await createPasskeyCredential(optionsData.data.options);
      const verifyResponse = await passkeyVerifyMutation.mutateAsync({
        body: {
          registration_session_id: flow.sessionId,
          challenge_token: optionsData.data.challenge_token,
          credential,
        },
      });
      const verifyData = verifyResponse as ApiResult<AuthResponse> | undefined;
      if (!verifyData?.data) {
        throw new Error(
          verifyData?.message || t("auth.register.passkeyVerifyError"),
        );
      }

      await completeAuth(verifyData.data);
      await queryClient.invalidateQueries({
        queryKey: bootstrapStatusQueryKey,
      });
      navigate(redirectTo, { replace: true });
    } catch (passkeyError) {
      setFlowError(
        getApiMessage(passkeyError, t("auth.register.passkeyVerifyError")),
      );
    }
  };

  const handleUseAuthenticatorApp = async () => {
    if (!flow) return;
    setFlowError(null);

    try {
      await startTotpSetup(flow.sessionId);
    } catch (totpError) {
      setFlowError(getApiMessage(totpError, t("auth.register.totpSetupStartError")));
    }
  };

  const handleCompleteTotp = async (
    event: FormEvent<HTMLFormElement>,
  ) => {
    event.preventDefault();
    if (!flow) return;

    setFlowError(null);

    try {
      const response = await totpCompleteMutation.mutateAsync({
        body: {
          registration_session_id: flow.sessionId,
          code: totpCode,
        },
      });
      const payload =
        response as ApiResult<RegistrationTOTPCompleteResponse> | undefined;
      if (!payload?.data?.auth) {
        throw new Error(
          payload?.message || t("auth.register.totpSetupCompleteError"),
        );
      }

      await completeAuth(payload.data.auth);
      await queryClient.invalidateQueries({
        queryKey: bootstrapStatusQueryKey,
      });
      setRecoveryCodes(payload.data.recovery_codes ?? []);
      setStep("recovery");
    } catch (totpError) {
      setFlowError(
        getApiMessage(totpError, t("auth.register.totpSetupCompleteError")),
      );
    }
  };

  const handleFinish = () => {
    navigate(redirectTo, { replace: true });
  };

  return {
    step,
    username,
    setUsername,
    password,
    setPassword,
    confirmPassword,
    setConfirmPassword,
    confirmPasswordRef,
    confirmPasswordMessage,
    capabilityMessage,
    totpSetup,
    totpCode,
    setTotpCode,
    recoveryCodes,
    qrCodeDataURL,
    displayError,
    isBusy,
    handleStartRegistration,
    handleCreatePasskey,
    handleUseAuthenticatorApp,
    handleCompleteTotp,
    handleFinish,
  };
}
