import { useState } from "react";
import { useNavigate } from "react-router-dom";
import type { RepositoryStorageStrategy } from "@/features/repositories";
import { usePreference } from "@/lib/preferences/preferences";
import { useRegistrationFlow } from "../registration/useRegistrationFlow.ts";

const FLOW_INDEX: Record<string, number> = {
  credentials: 1,
  totp: 2,
  passkey: 3,
  recovery: 4,
};

export function buildBootstrapPrimaryRepositoryRequest(
  name: string,
  storageStrategy: RepositoryStorageStrategy,
  assessmentToken?: string,
  acknowledgedRisks?: readonly string[],
) {
  return { name, storageStrategy, assessmentToken, acknowledgedRisks };
}

/**
 * Composes first-admin registration with the Primary Repository setup step.
 * The storage step itself is the shared primary-setup flow, which reads the
 * Server's review of the target and creates or adopts accordingly.
 */
export function useBootstrapFlow() {
  const navigate = useNavigate();
  const [welcomed, setWelcomed] = useState(false);
  const [mfaComplete, setMfaComplete] = useState(false);
  const [language, setLanguage] = usePreference("language");
  const [region, setRegion] = usePreference("region");

  const registration = useRegistrationFlow({ onComplete: () => setMfaComplete(true) });
  const current = mfaComplete ? 5 : welcomed ? (FLOW_INDEX[registration.step] ?? 1) : 0;

  return {
    ...registration,
    current,
    setWelcomed,
    language,
    setLanguage,
    region,
    setRegion,
    openDashboard: () => void navigate("/", { replace: true }),
  };
}
