import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQueryClient } from "@tanstack/react-query";
import {
  useSetupPrimaryRepository,
  validateRepositoryName,
  type RepositoryStorageStrategy,
} from "@/features/repositories";
import { useI18n } from "@/lib/i18n.tsx";
import { localizeProblem, localizeProblemType, normalizeProblem } from "@/lib/http-commons/problem";
import { setupStatusQueryKey } from "../../api/useSetupStatus.ts";
import {
  setupPrimaryTargetQueryKey,
  useSetupPrimaryTarget,
  type SetupPrimaryReview,
} from "../../api/useSetupPrimaryTarget.ts";

/** Problem types after which the review is fetched again instead of retried. */
const REVIEW_CHANGED_TYPES = new Set([
  "https://lumilio.org/problems/storage/confirmation-required",
  "https://lumilio.org/problems/storage/assessment-stale",
  "https://lumilio.org/problems/storage/layout-obstructed",
  "https://lumilio.org/problems/repository/conflict",
]);

export type PrimarySetupOutcome = "created" | "adopted";

/**
 * The one Primary Repository setup journey, shared by the bootstrap wizard
 * and the access gate (R3). The Server reviews the fixed target; this hook
 * only binds the administrator's decision to that review: create when the
 * target is creatable, adopt the Repository already there when it is
 * adoptable, otherwise point at the host's way to start fresh elsewhere.
 * Every acknowledgement is reset when the review token changes.
 */
export function usePrimarySetup(options: {
  onCompleted: (outcome: PrimarySetupOutcome) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const targetQuery = useSetupPrimaryTarget();
  const mutations = useSetupPrimaryRepository();

  const target = targetQuery.data;
  const review: SetupPrimaryReview | undefined = target?.target;
  const defaultStorage = target?.default_storage;
  const assessment = review?.assessment;
  const assessmentToken = assessment?.token;
  const placementRisks = useMemo(() => assessment?.risks ?? [], [assessment?.risks]);
  const actions = useMemo(() => review?.actions ?? [], [review?.actions]);

  const [name, setName] = useState("Primary Storage");
  const [storageStrategy, setStorageStrategy] = useState<RepositoryStorageStrategy>("date");
  const [riskAcknowledged, setRiskAcknowledged] = useState(false);
  useEffect(() => {
    setRiskAcknowledged(false);
  }, [assessmentToken]);

  const nameError = validateRepositoryName(name);
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
  const risksAcknowledged = placementRisks.length === 0 || riskAcknowledged;
  const canCreate = actions.includes("create");
  const canAdopt = actions.includes("adopt") && Boolean(review?.existing?.repository_id);
  const canSubmitCreate =
    canCreate && nameError === null && risksAcknowledged && !mutations.isPending;
  const canSubmitAdopt =
    canAdopt && Boolean(assessmentToken) && risksAcknowledged && !mutations.isPending;

  const refreshReview = async () => {
    setRiskAcknowledged(false);
    await queryClient.invalidateQueries({ queryKey: setupPrimaryTargetQueryKey });
  };
  const finish = async (outcome: PrimarySetupOutcome) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: setupStatusQueryKey }),
      queryClient.invalidateQueries({ queryKey: setupPrimaryTargetQueryKey }),
    ]);
    await options.onCompleted(outcome);
  };
  const handleFailure = async (error: unknown) => {
    const problem = normalizeProblem(error);
    if (problem.kind === "problem" && REVIEW_CHANGED_TYPES.has(problem.type)) {
      // The facts changed since the review was loaded: show the current ones.
      await refreshReview();
    }
  };

  const submitCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!canSubmitCreate) return;
    try {
      await mutations.createPrimaryRepository({
        name,
        storageStrategy,
        assessmentToken,
        acknowledgedRisks: placementRisks.length > 0 ? placementRisks : undefined,
      });
    } catch (error) {
      await handleFailure(error);
      return;
    }
    await finish("created");
  };

  const submitAdopt = async () => {
    if (!canSubmitAdopt || !review?.existing?.repository_id || !assessmentToken) return;
    try {
      await mutations.adoptPrimaryRepository({
        expectedRepositoryId: review.existing.repository_id,
        assessmentToken,
        acknowledgedRisks: placementRisks.length > 0 ? placementRisks : undefined,
      });
    } catch (error) {
      await handleFailure(error);
      return;
    }
    await finish("adopted");
  };

  const error = mutations.error
    ? localizeProblem(
        mutations.error,
        t,
        t("auth.primaryRepository.error", {
          defaultValue: "Failed to create the Primary Repository.",
        }),
      )
    : null;

  return {
    targetQuery,
    target,
    review,
    defaultStorage,
    nativeHost: target?.native_host ?? false,
    storageLocationPath: target?.repository_defaults?.storage_location ?? "",
    name,
    setName,
    nameError,
    storageStrategy,
    setStorageStrategy,
    placementRisks,
    riskAcknowledged,
    setRiskAcknowledged,
    reviewProblem,
    canCreate,
    canAdopt,
    canSubmitCreate,
    canSubmitAdopt,
    submitCreate,
    submitAdopt,
    refreshReview,
    isPending: mutations.isPending,
    error,
  };
}
