import { useEffect, useMemo, useRef, useState, type FormEvent, type RefObject } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { localizeProblem } from "@/lib/http-commons/problem";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "../../state/useAuth.ts";
import { setupStatusQueryKey } from "../../api/useSetupStatus.ts";
import { useBrowserCapabilities } from "../../api/useBrowserCapabilities.ts";
import type { TOTPSetupResponse } from "../../types.ts";
import { createPasskeyCredential, getPasskeySupport } from "../../modules/webauthn/webauthn.ts";
import { usePasswordConfirmation } from "../../hooks/usePasswordConfirmation.ts";

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
  passkeySecurityCode: string;
  setPasskeySecurityCode: (value: string) => void;
  recoveryCodes: string[];
  displayError: string | null;
  isBusy: boolean;
  handleStartRegistration: (event: FormEvent<HTMLFormElement>) => Promise<void>;
  handleCompleteTotp: () => Promise<void>;
  handleSkipTotp: () => void;
  handleCreatePasskey: () => Promise<void>;
  handleSkipPasskey: () => void;
  handleFinish: () => void;
};

export function useRegistrationFlow(options?: { onComplete?: () => void }): RegistrationFlowState {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { completeAuth, isAuthenticated } = useAuth();
  const registerMutation = $api.useMutation("post", "/api/v1/auth/register/start");
  const securityVerifyMutation = $api.useMutation("post", "/api/v1/auth/security/verify");
  const totpSetupMutation = $api.useMutation("post", "/api/v1/auth/mfa/totp/setup");
  const totpEnableMutation = $api.useMutation("post", "/api/v1/auth/mfa/totp/enable");
  const passkeyOptionsMutation = $api.useMutation("post", "/api/v1/auth/mfa/passkeys/options");
  const passkeyVerifyMutation = $api.useMutation("post", "/api/v1/auth/mfa/passkeys/verify");
  const browserCapabilities = useBrowserCapabilities();
  const location = useLocation();
  const navigate = useNavigate();
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
  // The code used to enable TOTP may expire while the browser ceremony is
  // waiting for WebAuthn. Keep a separate editable value so passkey setup can
  // be retried with a fresh factor instead of silently replaying stale input.
  const [passkeySecurityCode, setPasskeySecurityCode] = useState("");
  const [recoveryCodes, setRecoveryCodes] = useState<string[]>([]);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const passkeySupported =
    passkeySupport.supported && browserCapabilities.data?.passkey_available === true;
  const confirmPasswordMessage = t("auth.register.confirmPasswordHint", {
    defaultValue: "Passwords must match exactly.",
  });
  const confirmPasswordRef = usePasswordConfirmation(
    password,
    confirmPassword,
    confirmPasswordMessage,
  );
  const displayError = flowError;
  const isBusy =
    registerMutation.isPending ||
    securityVerifyMutation.isPending ||
    totpSetupMutation.isPending ||
    totpEnableMutation.isPending ||
    passkeyOptionsMutation.isPending ||
    passkeyVerifyMutation.isPending;

  useEffect(() => {
    // Bounce already-authenticated visitors away — but not the user who just
    // registered and is now stepping through the optional MFA setup.
    if (isAuthenticated && !startedRef.current) {
      void navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, navigate, redirectTo]);

  const beginTOTPSetup = async () => {
    const security = await securityVerifyMutation.mutateAsync({
      body: {
        current_password: password,
        purpose: "totp_setup",
      },
    });
    if (!security?.security_token) {
      throw new Error(
        t("auth.register.totpSecurityVerificationError", {
          defaultValue: "Unable to verify account security before TOTP setup.",
        }),
      );
    }

    const setupResponse = await totpSetupMutation.mutateAsync({
      body: { security_token: security.security_token },
    });
    const setupPayload = setupResponse;
    if (!setupPayload?.setup_token) {
      throw new Error(t("auth.register.totpSetupStartError"));
    }
    setTotpSetup(setupPayload);
    setTotpCode("");
    setStep("totp");
  };

  const handleStartRegistration = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFlowError(null);

    if (confirmPasswordRef.current && !confirmPasswordRef.current.checkValidity()) {
      confirmPasswordRef.current.reportValidity();
      return;
    }

    try {
      // Registration and the first security-verification request are separate
      // network operations. If the latter fails, the account already exists;
      // retry the ceremony instead of submitting registration a second time.
      if (startedRef.current) {
        await beginTOTPSetup();
        return;
      }

      const response = await registerMutation.mutateAsync({
        body: { username, password },
      });
      const payload = response;
      if (!payload) {
        throw new Error(t("auth.register.startError"));
      }

      // Account exists and is logged in. MFA is offered next but fully optional;
      // TOTP comes first because a passkey may only be added once TOTP is on.
      await completeAuth(payload);
      startedRef.current = true;
      await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
      await beginTOTPSetup();
    } catch (registrationError) {
      setFlowError(localizeProblem(registrationError, t, t("auth.register.startError")));
    }
  };

  const handleCompleteTotp = async () => {
    const setupToken = totpSetup?.setup_token;
    if (!setupToken) return;
    setFlowError(null);

    try {
      const response = await totpEnableMutation.mutateAsync({
        body: {
          setup_token: setupToken,
          code: totpCode,
        },
      });
      const payload = response;
      if (!payload) {
        throw new Error(t("auth.register.totpSetupCompleteError"));
      }

      if (payload.status?.totp_enabled !== true || !payload.session) {
        throw new Error(t("auth.register.totpSetupCompleteError"));
      }
      const codes = payload.recovery_codes ?? [];
      if (codes.length === 0) {
        throw new Error(t("auth.register.totpSetupCompleteError"));
      }
      await completeAuth(payload.session);
      // Enabling TOTP consumes this counter. Require a fresh code for the
      // purpose-bound passkey verification instead of prefilling a replay.
      setPasskeySecurityCode("");
      setRecoveryCodes(codes);
      // TOTP is now enabled, so a passkey may be offered as the next option.
      setStep(passkeySupported ? "passkey" : "recovery");
    } catch (totpError) {
      setTotpCode("");
      setFlowError(localizeProblem(totpError, t, t("auth.register.totpSetupCompleteError")));
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
      const security = await securityVerifyMutation.mutateAsync({
        body: {
          current_password: password,
          code: passkeySecurityCode,
          method: "totp",
          purpose: "passkey_mutation",
        },
      });
      if (!security?.security_token) {
        throw new Error(
          t("auth.register.passkeySecurityVerificationError", {
            defaultValue: "Unable to verify account security before passkey setup.",
          }),
        );
      }
      const optionsResponse = await passkeyOptionsMutation.mutateAsync({});
      const optionsData = optionsResponse;
      if (!optionsData?.challenge_token) {
        throw new Error(t("auth.register.passkeyStartError"));
      }

      const credential = await createPasskeyCredential(optionsData.options);
      const passkeyResponse = await passkeyVerifyMutation.mutateAsync({
        body: {
          challenge_token: optionsData.challenge_token,
          credential,
          security_token: security.security_token,
        },
      });
      if (!passkeyResponse?.session) {
        throw new Error(t("auth.register.passkeyVerifyError"));
      }
      await completeAuth(passkeyResponse.session);

      setStep("recovery");
    } catch (passkeyError) {
      setFlowError(localizeProblem(passkeyError, t, t("auth.register.passkeyVerifyError")));
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
    passkeySupported,
    totpSetup,
    totpCode,
    setTotpCode,
    passkeySecurityCode,
    setPasskeySecurityCode,
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
