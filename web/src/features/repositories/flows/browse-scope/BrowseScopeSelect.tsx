import { useI18n } from "@/lib/i18n";
import { useBrowseScope } from "./useBrowseScope";

export default function BrowseScopeSelect({ className = "" }: { className?: string }) {
  const { t } = useI18n();
  const { repositories, browseRepositoryId, setBrowseRepositoryId, getRepositoryLabel } =
    useBrowseScope();

  if (repositories.length === 0) return null;

  return (
    <select
      className={`select select-sm select-bordered rounded-full w-auto max-w-[14rem] ${className}`}
      value={browseRepositoryId}
      onChange={(e) => setBrowseRepositoryId(e.target.value || null)}
      title={t("assets.assetsPageHeader.scope.title", "Gallery scope")}
      aria-label={t("assets.assetsPageHeader.scope.title", "Gallery scope")}
    >
      <option value="">{t("navbar.repository.all", "All repositories")}</option>
      {repositories.map((repo) => (
        <option key={repo.id} value={repo.id}>
          {getRepositoryLabel(repo)}
        </option>
      ))}
    </select>
  );
}
