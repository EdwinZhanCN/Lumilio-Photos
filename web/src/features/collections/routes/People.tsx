import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Loader2, RefreshCw, Users } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { CollectionErrorAlert, LoadMoreButton } from "@/components/collection";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import {
  usePeople,
  useRebuildPeopleClusters,
} from "@/features/people/hooks/usePeople";
import { useWorkingRepository } from "@/features/settings";
import PeopleCollectionGrid from "../components/PeopleCollectionGrid";

const PAGE_SIZE = 24;

function PeopleContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const showMessage = useMessage();
  const { scopedRepositoryId } = useWorkingRepository();
  const [limit, setLimit] = useState(PAGE_SIZE);
  const { people, total, isLoading, isError, error, isFetching } = usePeople({
    limit,
    repositoryId: scopedRepositoryId,
  });
  const { rebuildPeople, isRebuilding } =
    useRebuildPeopleClusters(scopedRepositoryId);

  const handleRebuildPeople = async () => {
    try {
      const result = await rebuildPeople();
      showMessage(
        "success",
        t("people.rebuild.success", {
          clusters: result?.clusters_total ?? 0,
          faces: result?.clustered_faces ?? 0,
          noise: result?.noise_faces ?? 0,
        }),
      );
    } catch (err) {
      showMessage(
        "error",
        t("people.rebuild.error", {
          message: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  };

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.people")}
        icon={<Users className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <button
          type="button"
          className="btn btn-primary btn-sm rounded-full"
          onClick={handleRebuildPeople}
          disabled={isRebuilding}
        >
          {isRebuilding ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <RefreshCw className="size-4" />
          )}
          {isRebuilding
            ? t("people.rebuild.running")
            : t("people.rebuild.action")}
        </button>
      </PageHeader>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {isError && (
            <CollectionErrorAlert
              message={t("collections.messages.loadPeopleError", {
                message:
                  error instanceof Error ? error.message : t("home.errors.unknown"),
              })}
            />
          )}

          <PeopleCollectionGrid
            people={people}
            loading={isLoading}
            repositoryId={scopedRepositoryId}
            onPersonClick={(person) => {
              if (!person?.person_id) return;
              void navigate(`/people/${person.person_id}`);
            }}
          />

          {total > people.length && (
            <LoadMoreButton
              onClick={() => setLimit((current) => current + PAGE_SIZE)}
              loading={isFetching}
              className=""
            />
          )}
        </div>
      </div>
    </div>
  );
}

export default function People() {
  const { t } = useI18n();

  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack
          code={500}
          title={t("assets.errorFallback.something_went_wrong")}
          {...props}
        />
      )}
    >
      <PeopleContent />
    </ErrorBoundary>
  );
}
