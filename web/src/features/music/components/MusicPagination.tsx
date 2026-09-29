import { useI18n } from "@/lib/i18n";

type MusicPaginationProps = {
  offset: number;
  pageSize: number;
  total: number;
  isFetching?: boolean;
  onPrevious: () => void;
  onNext: () => void;
};

/**
 * Shared Previous / Next pager for the detail track lists (Album / Playlist /
 * Artist). Mirrors the browse Tracks pager so every list pages identically.
 */
export default function MusicPagination({
  offset,
  pageSize,
  total,
  isFetching,
  onPrevious,
  onNext,
}: MusicPaginationProps) {
  const { t } = useI18n();
  const start = Math.min(offset + 1, total);
  const end = Math.min(offset + pageSize, total);
  return (
    <div className="mt-4 flex items-center justify-between gap-3">
      <button
        type="button"
        className="btn btn-sm"
        disabled={offset === 0 || isFetching}
        onClick={onPrevious}
      >
        {t("common.previous", "Previous")}
      </button>
      <span className="text-sm text-base-content/60">
        {start}–{end} / {total}
      </span>
      <button
        type="button"
        className="btn btn-sm"
        disabled={end >= total || isFetching}
        onClick={onNext}
      >
        {t("common.next", "Next")}
      </button>
    </div>
  );
}
