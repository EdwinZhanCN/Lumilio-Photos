import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import {
  useSetupPrimaryRepository,
  validateRepositoryName,
  type RepositoryStorageStrategy,
} from "@/features/repositories";
import { useI18n } from "@/lib/i18n.tsx";
import { localizeProblem, localizeProblemType, normalizeProblem } from "@/lib/http-commons/problem";
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
  assessmentToken?: string,
  acknowledgedRisks?: readonly string[],
) {
  return { name, storageStrategy, assessmentToken, acknowledgedRisks };
}

export function useBootstrapFlow() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const setupQuery = useSetupStatus();
  const createRepositoryMutation = useSetupPrimaryRepository();

  const [welcomed, setWelcomed] = useState(false);
  const [mfaComplete, setMfaComplete] = useState(false);
  const [language, setLanguage] = usePreference("language");
  const [region, setRegion] = usePreference("region");
  const defaults = setupQuery.data?.repository_defaults;
  const [repoName, setRepoName] = useState("Primary Storage");
  const [defaultStorageLocation, setDefaultStorageLocation] = useState("");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskAcknowledged, setRiskAcknowledged] = useState(false);
  // The Server's review of <default>/primary; its token binds the acknowledgement.
  const assessment = defaults?.assessment;
  const placementRisks = useMemo(() => assessment?.risks ?? [], [assessment?.risks]);
  const assessmentToken = assessment?.token;
  const reviewProblem = assessment?.problem
    ? localizeProblemType(
        `https://lumilio.org/problems/${assessment.problem}`,
        t,
        t("auth.primaryRepository.targetUnavailable", {
          defaultValue:
            "The Default Storage Location cannot hold the Primary Repository right now.",
        }),
      )
    : null;

  useEffect(() => {
    if (!defaults) return;
    setDefaultStorageLocation((current) => current || defaults.storage_location || "");
  }, [defaults]);

  useEffect(() => {
    setRiskAcknowledged(false);
  }, [assessmentToken]);

  const registration = useRegistrationFlow({ onComplete: () => setMfaComplete(true) });
  const current = mfaComplete ? 5 : welcomed ? (FLOW_INDEX[registration.step] ?? 1) : 0;
  const repoNameError = validateRepositoryName(repoName);
  const canSubmitRepo = useMemo(
    () =>
      repoNameError === null &&
      !reviewProblem &&
      (placementRisks.length === 0 || riskAcknowledged) &&
      !createRepositoryMutation.isPending,
    [
      createRepositoryMutation.isPending,
      placementRisks.length,
      repoNameError,
      reviewProblem,
      riskAcknowledged,
    ],
  );

  const submitRepo = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitRepo) return;

    try {
      await createRepositoryMutation.createPrimaryRepository(
        buildBootstrapPrimaryRepositoryRequest(
          repoName,
          storageStrategy,
          assessmentToken,
          placementRisks.length > 0 ? placementRisks : undefined,
        ),
      );
    } catch (error) {
      const problem = normalizeProblem(error);
      if (
        problem.kind === "problem" &&
        (problem.type === "https://lumilio.org/problems/storage/confirmation-required" ||
          problem.type === "https://lumilio.org/problems/storage/assessment-stale")
      ) {
        // Facts changed since the status was loaded: fetch the current review.
        setRiskAcknowledged(false);
        await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
      }
      return;
    }
    await queryClient.invalidateQueries({ queryKey: setupStatusQueryKey });
    void navigate("/", { replace: true });
  };

  const repoError = createRepositoryMutation.error
    ? localizeProblem(
        createRepositoryMutation.error,
        t,
        t("auth.primaryRepository.error", {
          defaultValue: "Failed to create the Primary Repository.",
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
    defaultStorageLocation,
    storageStrategy,
    setStorageStrategy,
    placementRisks,
    reviewProblem,
    riskAcknowledged,
    setRiskAcknowledged,
    canSubmitRepo,
    submitRepo,
    repoError,
    isCreatingRepository: createRepositoryMutation.isPending,
  };
}
