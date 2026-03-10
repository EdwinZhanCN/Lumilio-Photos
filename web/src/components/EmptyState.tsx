import { Image as ImageIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n";

interface EmptyStateProps {
  className?: string;
  title?: string;
  description?: string;
}

function EmptyState({
  className = "",
  title,
  description,
}: EmptyStateProps) {
  const { t } = useI18n();

  const resolvedTitle = title
    ? title
    : t("common.emptyState.title", {
        defaultValue: "Nothing here yet",
      });
  const resolvedDescription = description
    ? description
    : t("common.emptyState.description", {
        defaultValue: "Items will appear here when available.",
      });

  return (
    <div
      className={`flex min-h-[400px] w-full items-center justify-center p-6 ${className}`}
    >
      <div className="flex w-full max-w-md flex-col items-center gap-4 rounded-[2rem] border border-dashed border-base-300 bg-base-200/40 px-8 py-12 text-center">
        <div className="flex size-16 items-center justify-center rounded-2xl border border-base-300 bg-base-100 text-base-content/50">
          <ImageIcon className="size-8" strokeWidth={1.75} />
        </div>
        <div className="space-y-2">
          <h3 className="text-lg font-semibold text-base-content">
            {resolvedTitle}
          </h3>
          <p className="text-sm leading-6 text-base-content/60">
            {resolvedDescription}
          </p>
        </div>
      </div>
    </div>
  );
}

export default EmptyState;
