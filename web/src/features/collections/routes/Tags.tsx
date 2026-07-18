import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Tag as TagIcon, Search } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import PageHeader from "@/components/ui/PageHeader";
import { BrowseScopeSelect, useBrowseScope } from "@/features/repositories";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import { assetUrls } from "@/lib/assets/assetUrls";
import RailCard from "../components/RailCard";
import { useTagSummaries } from "../api/useTagSummaries";
import { encodeTagKey } from "../utils/tagKey";

type SourceFilter = "all" | "user" | "zeroshot";

function TagsContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const { scopedRepositoryId } = useBrowseScope();
  const [search, setSearch] = useState("");
  const [sourceFilter, setSourceFilter] = useState<SourceFilter>("all");

  const { data, isPending } = useTagSummaries({
    repositoryId: scopedRepositoryId,
    source: sourceFilter === "all" ? undefined : sourceFilter,
    query: search.trim() || undefined,
  });
  const tags = data?.tags ?? [];

  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.tags", "Tags") },
  ]);

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.tags", "Tags")}
        icon={<TagIcon className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      >
        <BrowseScopeSelect />
      </PageHeader>

      <div className="flex flex-wrap items-center gap-3 px-4 pb-2">
        <label className="input input-sm input-bordered flex items-center gap-2">
          <Search className="h-3.5 w-3.5 opacity-60" />
          <input
            type="text"
            className="grow"
            placeholder={t("collections.tags.searchPlaceholder", "Search tags")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />
        </label>

        <div className="join">
          <button
            type="button"
            className={`btn btn-sm join-item ${sourceFilter === "all" ? "btn-active" : ""}`}
            onClick={() => setSourceFilter("all")}
          >
            {t("collections.tags.sourceAll", "All")}
          </button>
          <button
            type="button"
            className={`btn btn-sm join-item ${sourceFilter === "user" ? "btn-active" : ""}`}
            onClick={() => setSourceFilter("user")}
          >
            {t("collections.tags.sourceManual", "Manual")}
          </button>
          <button
            type="button"
            className={`btn btn-sm join-item ${sourceFilter === "zeroshot" ? "btn-active" : ""}`}
            onClick={() => setSourceFilter("zeroshot")}
          >
            {t("collections.tags.sourceAI", "AI")}
          </button>
        </div>
      </div>

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-2">
        {!isPending && tags.length === 0 ? (
          <p className="text-sm text-base-content/60">
            {t("collections.tags.empty", "No tags found.")}
          </p>
        ) : (
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
            {tags.map((tag) => (
              <RailCard
                key={`${tag.tag_id}:${tag.source}`}
                media={{
                  kind: "photo",
                  src: tag.cover_asset_id
                    ? assetUrls.getThumbnailUrl(tag.cover_asset_id, "medium")
                    : null,
                  fallbackIcon: TagIcon,
                }}
                title={tag.tag_name || ""}
                subtitle={t("collections.folders.itemCount", {
                  count: tag.asset_count ?? 0,
                  defaultValue: "{{count}} items",
                })}
                onClick={() =>
                  navigate(
                    `/collections/tags/${encodeTagKey({
                      tagName: tag.tag_name ?? "",
                      source: tag.source ?? "",
                    })}`,
                  )
                }
                className="w-full"
              />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default function Tags() {
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
      <TagsContent />
    </ErrorBoundary>
  );
}
