import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Users } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import PageHeader from "@/components/ui/PageHeader";
import { BrowseScopeSelect, useBrowseScope } from "@/features/repositories";
import { CollectionErrorAlert, LoadMoreButton } from "@/components/collection";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { usePeople } from "@/features/people/hooks/usePeople";
import PeopleCollectionGrid from "../components/PeopleCollectionGrid";

const PAGE_SIZE = 24;

function PeopleContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.people", "People") },
  ]);
  const { scopedRepositoryId } = useBrowseScope();
  const [limit, setLimit] = useState(PAGE_SIZE);
  const [includeHidden, setIncludeHidden] = useState(false);
  const { people, total, isLoading, isError, error, isFetching } = usePeople({
    limit,
    repositoryId: scopedRepositoryId,
    includeHidden,
  });

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.people")}
        icon={<Users className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
        <div className="join">
          <button
            type="button"
            className={`btn btn-sm join-item ${includeHidden ? "btn-ghost" : "btn-active"}`}
            onClick={() => {
              setIncludeHidden(false);
              setLimit(PAGE_SIZE);
            }}
          >
            {t("people.hidden.visibleTab", "Visible")}
          </button>
          <button
            type="button"
            className={`btn btn-sm join-item ${includeHidden ? "btn-active" : "btn-ghost"}`}
            onClick={() => {
              setIncludeHidden(true);
              setLimit(PAGE_SIZE);
            }}
          >
            {t("people.hidden.allTab", "All")}
          </button>
        </div>
      </PageHeader>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="space-y-6">
          {isError && (
            <CollectionErrorAlert
              message={t("collections.messages.loadPeopleError", {
                message: error instanceof Error ? error.message : t("home.errors.unknown"),
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
        <ErrorFallback
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
