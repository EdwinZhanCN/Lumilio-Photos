import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useCreateRepository } from "@/features/repositories";
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

const isStorageStrategy = (value?: string): value is "cas" | "date" | "flat" =>
  value === "cas" || value === "date" || value === "flat";

const isDuplicateHandling = (value?: string): value is "overwrite" | "rename" | "uuid" =>
  value === "overwrite" || value === "rename" || value === "uuid";

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
  const [strategy, setStrategy] = useState<"cas" | "date" | "flat">("date");
  const [duplicateHandling, setDuplicateHandling] = useState<"overwrite" | "rename" | "uuid">(
    "rename",
  );

  useEffect(() => {
    if (!defaults) return;
    setRepoRoot((current) => current || defaults.default_root || "");
    setStrategy(isStorageStrategy(defaults.strategy) ? defaults.strategy : "date");
    setDuplicateHandling(
      isDuplicateHandling(defaults.duplicate_handling) ? defaults.duplicate_handling : "rename",
    );
  }, [defaults]);

  const registration = useRegistrationFlow({ onComplete: () => setMfaComplete(true) });
  const current = mfaComplete ? 5 : welcomed ? (FLOW_INDEX[registration.step] ?? 1) : 0;
  const canSubmitRepo = useMemo(
    () =>
      repoName.trim() !== "" && repoRoot.trim() !== "" && !createRepositoryMutation.isPending,
    [createRepositoryMutation.isPending, repoName, repoRoot],
  );

  const submitRepo = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitRepo) return;

    await createRepositoryMutation.createRepository({
      name: repoName.trim(),
      role: "primary",
      storageStrategy: strategy,
      duplicateHandling,
    });
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
    repoRoot,
    strategy,
    setStrategy,
    duplicateHandling,
    setDuplicateHandling,
    canSubmitRepo,
    submitRepo,
    repoError,
    isCreatingRepository: createRepositoryMutation.isPending,
  };
}
