import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  useCreateRepository,
  validateRepositoryName,
  type RepositoryStorageStrategy,
} from "@/features/repositories";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "@/lib/preferences/preferences";
import { setupStatusQueryKey, useSetupStatus } from "../../api/useSetupStatus.ts";
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
  riskConfirmation?: boolean,
) {
  return { name, role: "primary" as const, storageStrategy, riskConfirmation };
}

function apiMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const record = error as { message?: string; error?: string };
    return record.message || record.error || fallback;
  }
  return fallback;
}

export function useBootstrapFlow() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const createRepositoryMutation = useCreateRepository();

  const [welcomed, setWelcomed] = useState(false);
  const [mfaComplete, setMfaComplete] = useState(false);
  const [language, setLanguage] = usePreference("language");
  const [region, setRegion] = usePreference("region");
  const defaults = setupQuery.data?.repository_defaults;
  const [repoName, setRepoName] = useState("Primary Storage");
  const [repoRoot, setRepoRoot] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskConfirmation, setRiskConfirmation] = useState(false);
  const placementRisks = defaults?.risk_warnings ?? [];

  useEffect(() => {
    if (!defaults) return;
    setRepoRoot((current) => current || defaults.default_root || "");
  }, [defaults]);

  const registration = useRegistrationFlow({ onComplete: () => setMfaComplete(true) });
  const current = mfaComplete ? 5 : welcomed ? (FLOW_INDEX[registration.step] ?? 1) : 0;
  const repoNameError = validateRepositoryName(repoName);
  const canSubmitRepo = useMemo(
    () =>
      repoNameError === null &&
      repoRoot.trim() !== "" &&
      (placementRisks.length === 0 || riskConfirmation) &&
      !createRepositoryMutation.isPending,
    [
      createRepositoryMutation.isPending,
      placementRisks.length,
      repoNameError,
      repoRoot,
      riskConfirmation,
    ],
  );

  const submitRepo = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitRepo) return;

    await createRepositoryMutation.createRepository(
      buildBootstrapPrimaryRepositoryRequest(
        repoName,
        storageStrategy,
        placementRisks.length > 0 ? riskConfirmation : undefined,
      ),
    );
    await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
    void navigate("/", { replace: true });
  };

  const repoError = createRepositoryMutation.error
    ? apiMessage(
        createRepositoryMutation.error,
        t("auth.primaryRepository.error", {
          defaultValue: "Failed to create the primary repository.",
        }),
      )
    : null;

  return {
    ...registration,
    current,
    setWelcomed,
    language,
    setLanguage,
    region,
    setRegion,
    repoName,
    setRepoName,
    repoNameError,
    repoRoot,
    storageStrategy,
    setStorageStrategy,
    placementRisks,
    riskConfirmation,
    setRiskConfirmation,
    canSubmitRepo,
    submitRepo,
    repoError,
    isCreatingRepository: createRepositoryMutation.isPending,
  };
}
