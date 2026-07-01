import { useEffect, useMemo, useRef, useState, type FormEvent, type RefObject } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "./useAuth.ts";
import { setupStatusQueryKey } from "./useSetupStatus.ts";
import type {
  AuthResponse,
  PasskeyOptionsResponse,
  RecoveryCodesResponse,
  TOTPSetupResponse,
} from "../auth.type.ts";
import { createPasskeyCredential, getPasskeySupport } from "../lib/webauthn.ts";

type AuthRedirectState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

export type RegistrationFlowStep = "credentials" | "totp" | "passkey" | "recovery";

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
  passkeySupported: boolean;
  totpSetup: TOTPSetupResponse | null;
  totpCode: string;
  setTotpCode: (value: string) => void;
  recoveryCodes: string[];
  displayError: string | null;
  isBusy: boolean;
  handleStartRegistration: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  handleCompleteTotp: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  handleSkipTotp: () => void;
  handleCreatePasskey: () => Promise<void>;
  handleSkipPasskey: () => void;
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

export function useRegistrationFlow(options?: { onComplete?: () => void }): RegistrationFlowState {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { completeAuth, isAuthenticated } = useAuth();
  const registerMutation = $api.useMutation("post", "/api/v1/auth/register/start");
  const totpSetupMutation = $api.useMutation("post", "/api/v1/auth/mfa/totp/setup");
  const totpEnableMutation = $api.useMutation("post", "/api/v1/auth/mfa/totp/enable");
  const passkeyOptionsMutation = $api.useMutation("post", "/api/v1/auth/mfa/passkeys/options");
  const passkeyVerifyMutation = $api.useMutation("post", "/api/v1/auth/mfa/passkeys/verify");
  const location = useLocation();
  const navigate = useNavigate();
  const confirmPasswordRef = useRef<HTMLInputElement | null>(null);
  // Set once the account is created so the redirect effect doesn't bounce the
  // freshly-registered (now authenticated) user out of the optional MFA steps.
  const startedRef = useRef(false);

  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [step, setStep] = useState<RegistrationFlowStep>("credentials");
  const [flowError, setFlowError] = useState<string | null>(null);
  const [totpSetup, setTotpSetup] = useState<TOTPSetupResponse | null>(null);
  const [totpCode, setTotpCode] = useState("");
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const confirmPasswordMessage = t("auth.register.confirmPasswordHint", {
    defaultValue: "Passwords must match exactly.",
  });
  const displayError = flowError;
  const isBusy =
    registerMutation.isPending ||
    totpSetupMutation.isPending ||
    totpEnableMutation.isPending ||
    passkeyOptionsMutation.isPending ||
    passkeyVerifyMutation.isPending;

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
    // Bounce already-authenticated visitors away — but not the user who just
    // registered and is now stepping through the optional MFA setup.
    if (isAuthenticated && !startedRef.current) {
      void navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, navigate, redirectTo]);

  const handleStartRegistration = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFlowError(null);

    if (confirmPasswordRef.current && !confirmPasswordRef.current.checkValidity()) {
      confirmPasswordRef.current.reportValidity();
      return;
    }

    try {
      const response = await registerMutation.mutateAsync({
        body: { username, password },
      });
      const payload = response as AuthResponse | undefined;
      if (!payload) {
        throw new Error(t("auth.register.startError"));
      }

      // Account exists and is logged in. MFA is offered next but fully optional;
      // TOTP comes first because a passkey may only be added once TOTP is on.
      startedRef.current = true;
      await completeAuth(payload);
      await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });

      const setupResponse = await totpSetupMutation.mutateAsync({});
      const setupPayload = setupResponse as TOTPSetupResponse | undefined;
      if (!setupPayload) {
        throw new Error(t("auth.register.totpSetupStartError"));
      }
      setTotpSetup(setupPayload);
      setTotpCode("");
      setStep("totp");
    } catch (registrationError) {
      setFlowError(getApiMessage(registrationError, t("auth.register.startError")));
    }
  };

  const handleCompleteTotp = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!totpSetup) return;
    setFlowError(null);

    try {
      const response = await totpEnableMutation.mutateAsync({
        body: {
          setup_token: totpSetup.setup_token,
          code: totpCode,
        },
      });
      const payload = response as RecoveryCodesResponse | undefined;
      if (!payload) {
        throw new Error(t("auth.register.totpSetupCompleteError"));
      }

      setRecoveryCodes(payload.recovery_codes ?? []);
      // TOTP is now enabled, so a passkey may be offered as the next option.
      setStep(passkeySupport.supported ? "passkey" : "recovery");
    } catch (totpError) {
      setTotpCode("");
      setFlowError(getApiMessage(totpError, t("auth.register.totpSetupCompleteError")));
    }
  };

  // Skipping TOTP skips all MFA — the account stays password-only.
  const handleSkipTotp = () => {
    if (options?.onComplete) {
      options.onComplete();
      return;
    }
    void navigate(redirectTo, { replace: true });
  };

  const handleCreatePasskey = async () => {
    setFlowError(null);
    try {
      const optionsResponse = await passkeyOptionsMutation.mutateAsync({});
      const optionsData = optionsResponse as PasskeyOptionsResponse | undefined;
      if (!optionsData) {
        throw new Error(t("auth.register.passkeyStartError"));
      }

      const credential = await createPasskeyCredential(optionsData.options);
      await passkeyVerifyMutation.mutateAsync({
        body: {
          challenge_token: optionsData.challenge_token,
          credential,
        },
      });

      setStep("recovery");
    } catch (passkeyError) {
      setFlowError(getApiMessage(passkeyError, t("auth.register.passkeyVerifyError")));
    }
  };

  const handleSkipPasskey = () => {
    setStep("recovery");
  };

  const handleFinish = () => {
    if (options?.onComplete) {
      options.onComplete();
      return;
    }
    void navigate(redirectTo, { replace: true });
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
    passkeySupported: passkeySupport.supported,
    totpSetup,
    totpCode,
    setTotpCode,
    recoveryCodes,
    displayError,
    isBusy,
    handleStartRegistration,
    handleCompleteTotp,
    handleSkipTotp,
    handleCreatePasskey,
    handleSkipPasskey,
    handleFinish,
  };
}
