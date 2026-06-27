import type { ReactNode } from "react";
import { useI18n } from "@/lib/i18n.tsx";

/**
 * Centered "load more" pager button shared by the collection list pages
 * (Albums, People) so the label and styling stay identical.
 */
export function LoadMoreButton({
  onClick,
  loading = false,
  className = "py-8",
}: {
  onClick: () => void;
  loading?: boolean;
  className?: string;
}): ReactNode {
  const { t } = useI18n();
  return (
    <div className={`flex justify-center ${className}`}>
      <button
        type="button"
        onClick={onClick}
        disabled={loading}
        className="btn btn-outline btn-wide"
      >
        {loading ? t("common.loading") : t("common.loadMore")}
      </button>
    </div>
  );
}

export default LoadMoreButton;
