import { useEffect, useMemo, useRef, useState, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { USERNAME_MIN_LENGTH } from "../../model/credentialPolicy.ts";
import { getPasskeyCredential, getPasskeySupport } from "../../modules/webauthn/webauthn.ts";
import { storeRequiredPasswordChangeChallenge } from "../../state/passwordChangeChallenge.ts";
import { useAuth } from "../../state/useAuth.ts";
import type { MFAMethod, User } from "../../types.ts";

type AuthRedirectState = {
  from?: {
    pathname?: string;
    search?: string;
    hash?: string;
  };
};

type LoginChallenge = {
  user: User | null;
  mfaToken: string;
  mfaMethods: MFAMethod[];
};

export type LoginStep = "identify" | "passkey" | "password" | "mfa";

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

export function useLoginFlow() {
  const { t } = useI18n();
  const { login, verifyMFA, completeAuth, dispatch, isAuthenticated, isLoading, error } = useAuth();
  const loginOptionsMutation = $api.useMutation("post", "/api/v1/auth/login/options");
  const passkeyOptionsMutation = $api.useMutation("post", "/api/v1/auth/passkeys/login/options");
  const passkeyVerifyMutation = $api.useMutation("post", "/api/v1/auth/passkeys/login/verify");
  const location = useLocation();
  const navigate = useNavigate();

  const [step, setStep] = useState<LoginStep>("identify");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [challenge, setChallenge] = useState<LoginChallenge | null>(null);
  const [mfaCode, setMfaCode] = useState("");
  const [mfaMethod, setMfaMethod] = useState<MFAMethod>("totp");
  const [passkeyError, setPasskeyError] = useState<string | null>(null);
  const [optionsError, setOptionsError] = useState<string | null>(null);
  const [passkeyUnsupportedNote, setPasskeyUnsupportedNote] = useState<string | null>(null);
  const passwordInputRef = useRef<HTMLInputElement>(null);

  const redirectTo = useMemo(() => {
    const from = (location.state as AuthRedirectState | null)?.from;
    if (!from?.pathname) return "/";
    return `${from.pathname}${from.search ?? ""}${from.hash ?? ""}`;
  }, [location.state]);

  const passkeySupport = useMemo(() => getPasskeySupport(), []);
  const displayName = challenge?.user?.display_name || challenge?.user?.username || username;
  const recoveryCodeAvailable = challenge?.mfaMethods.includes("recovery_code") ?? false;
  const passkeyBusy = passkeyOptionsMutation.isPending || passkeyVerifyMutation.isPending;
  const identifyBusy = loginOptionsMutation.isPending;
  const displayError = optionsError ?? passkeyError ?? error;
  const passkeySupportReason = passkeySupport.reasonKey ? t(passkeySupport.reasonKey) : null;
  const usernameValid = username.trim().length >= USERNAME_MIN_LENGTH;

  useEffect(() => {
    if (!isLoading && isAuthenticated) {
      void navigate(redirectTo, { replace: true });
    }
  }, [isAuthenticated, isLoading, navigate, redirectTo]);

  useEffect(() => {
    if (step !== "password") return;
    passwordInputRef.current?.focus();
  }, [step]);

  const clearStepErrors = () => {
    setPasskeyError(null);
    setOptionsError(null);
    dispatch({ type: "AUTH_IDLE" });
  };

  const goToIdentify = () => {
    setStep("identify");
    setPassword("");
    setPasskeyUnsupportedNote(null);
    setChallenge(null);
    setMfaCode("");
    setMfaMethod("totp");
    clearStepErrors();
  };

  const goToPassword = (note: string | null = null) => {
    setStep("password");
    setPassword("");
    setPasskeyUnsupportedNote(note);
    clearStepErrors();
  };

  const handleIdentify = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    clearStepErrors();
    setPasskeyUnsupportedNote(null);

    if (!usernameValid) {
      setOptionsError(
        t("auth.login.usernameRequired", {
          defaultValue: "Enter your username to continue.",
        }),
      );
      return;
    }

    try {
      const options = await loginOptionsMutation.mutateAsync({
        body: { username },
      });
      if (!options) {
        throw new Error(
          t("auth.login.optionsError", {
            defaultValue: "Unable to continue with this username.",
          }),
        );
      }

      if (options.passkey && passkeySupport.supported) {
        setStep("passkey");
        return;
      }

      const note = options.passkey && !passkeySupport.supported ? passkeySupportReason : null;
      goToPassword(note);
    } catch (identifyError) {
      setOptionsError(
        getApiMessage(
          identifyError,
          t("auth.login.optionsError", {
            defaultValue: "Unable to continue with this username.",
          }),
        ),
      );
    }
  };

  const handlePasswordLogin = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    setPasskeyError(null);
    setOptionsError(null);
    try {
      const result = await login(username, password);
      if (result.status === "password_change_required") {
        storeRequiredPasswordChangeChallenge({
          passwordChangeToken: result.challenge.passwordChangeToken,
          username: result.challenge.user?.username ?? username,
          redirectTo,
        });
        void navigate("/password-change-required", { replace: true });
        return;
      }
      if (result.status === "mfa_required") {
        setChallenge(result.challenge);
        setMfaMethod(
          result.challenge.mfaMethods.includes("totp")
            ? "totp"
            : (result.challenge.mfaMethods[0] ?? "totp"),
        );
        setMfaCode("");
        setStep("mfa");
        return;
      }
      void navigate(redirectTo, { replace: true });
    } catch {
      // Auth context owns password errors.
    }
  };

  const handlePasskeyLogin = async () => {
    setPasskeyError(null);
    setOptionsError(null);

    try {
      const optionsData = await passkeyOptionsMutation.mutateAsync({
        body: { username },
      });
      if (!optionsData?.challenge_token) {
        throw new Error(t("auth.login.passkeyStartError"));
      }

      const credential = await getPasskeyCredential(optionsData.options);
      const verifyData = await passkeyVerifyMutation.mutateAsync({
        body: {
          challenge_token: optionsData.challenge_token,
          credential,
        },
      });
      if (!verifyData) {
        throw new Error(t("auth.login.passkeyVerifyError"));
      }

      await completeAuth(verifyData);
      void navigate(redirectTo, { replace: true });
    } catch (passkeyAuthError) {
      setPasskeyError(getApiMessage(passkeyAuthError, t("auth.login.passkeyUnavailable")));
    }
  };

  const handleVerifyMFA = async (code?: string) => {
    if (!challenge) return;
    const value = code ?? mfaCode;
    try {
      await verifyMFA(challenge.mfaToken, value, mfaMethod);
      void navigate(redirectTo, { replace: true });
    } catch {
      setMfaCode("");
      // Auth context owns MFA errors.
    }
  };

  const handleBackFromMFA = () => {
    setChallenge(null);
    setMfaCode("");
    setMfaMethod("totp");
    setStep("password");
    clearStepErrors();
  };

  const toggleMFAMethod = () => {
    setMfaMethod((method) => (method === "totp" ? "recovery_code" : "totp"));
    setMfaCode("");
  };

  return {
    step,
    challenge,
    username,
    setUsername,
    password,
    setPassword,
    mfaCode,
    setMfaCode,
    mfaMethod,
    displayName,
    recoveryCodeAvailable,
    passkeyBusy,
    identifyBusy,
    displayError,
    usernameValid,
    isLoading,
    passkeyUnsupportedNote,
    passwordInputRef,
    registrationState: location.state,
    goToIdentify,
    goToPassword,
    handleIdentify,
    handlePasswordLogin,
    handlePasskeyLogin,
    handleVerifyMFA,
    handleBackFromMFA,
    toggleMFAMethod,
  };
}

export type LoginFlowState = ReturnType<typeof useLoginFlow>;
