import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { AlertTriangle, Users } from "lucide-react";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { usePeople } from "@/features/people/hooks/usePeople";
import { useWorkingRepository } from "@/features/settings";
import PeopleCollectionGrid from "../components/PeopleCollectionGrid";

const PAGE_SIZE = 24;

function PeopleContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { scopedRepositoryId } = useWorkingRepository();
  const [limit, setLimit] = useState(PAGE_SIZE);
  const { people, total, isLoading, isError, error, isFetching } = usePeople({
    limit,
    repositoryId: scopedRepositoryId,
  });

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.people")}
        icon={<Users className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      />

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {isError && (
            <div className="alert alert-warning">
              <AlertTriangle className="size-5" />
              <span>
                {t("collections.messages.loadPeopleError", {
                  message:
                    error instanceof Error
                      ? error.message
                      : t("home.errors.unknown"),
                })}
              </span>
            </div>
          )}

          <PeopleCollectionGrid
            people={people}
            loading={isLoading}
            repositoryId={scopedRepositoryId}
            onPersonClick={(person) => {
              if (!person?.person_id) return;
              navigate(`/people/${person.person_id}?groupBy=date`);
            }}
          />

          {total > people.length && (
            <div className="flex justify-center">
              <button
                type="button"
                className="btn btn-outline btn-wide"
                onClick={() => setLimit((current) => current + PAGE_SIZE)}
                disabled={isFetching}
              >
                {isFetching ? t("common.loading") : t("common.loadMore")}
              </button>
            </div>
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
