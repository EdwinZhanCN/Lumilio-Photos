import { BadgeCheck, Hammer, Loader, TriangleAlert, X } from "lucide-react";
import { useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import type { ToolBlock } from "../../types";

interface ToolCallBlockProps {
  block: ToolBlock;
}

/** One tool execution as a status chip: spinner while running, the receipt
 * summary on success, the typed error (with recovery hint) on failure.
 * An empty result set is surfaced loudly — INV-3's UI face. */
export function ToolCallBlock({ block }: ToolCallBlockProps) {
  const { t } = useI18n();
  const [showDetail, setShowDetail] = useState(false);

  const isEmptyResult =
    block.status === "success" && block.refId !== undefined && block.count === 0;

  const accent =
    block.status === "error"
      ? "border-error/30 text-error"
      : isEmptyResult
        ? "border-warning/40 text-warning"
        : block.status === "success"
          ? "border-success/30 text-success"
          : "border-info/30 text-info";

  const summary = isEmptyResult
    ? t("lumilio.tools.emptyResult", "No matching photos")
    : (block.message ?? block.error?.message);

  return (
    <div className="my-2">
      <button
        onClick={() => setShowDetail(!showDetail)}
        className={`inline-flex max-w-full items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-medium transition-colors duration-200 cursor-pointer ${accent}`}
      >
        <Hammer className="flex-shrink-0" size={12} strokeWidth={1.5} />
        <span className="flex-shrink-0 whitespace-nowrap">{block.name}</span>
        {block.status === "running" && <Loader className="flex-shrink-0 animate-spin" size={13} />}
        {block.status === "success" &&
          (isEmptyResult ? (
            <TriangleAlert className="flex-shrink-0" size={13} />
          ) : (
            <BadgeCheck className="flex-shrink-0" size={13} />
          ))}
        {block.status === "error" && <X className="flex-shrink-0" size={13} />}
        {summary && (
          <span className="min-w-0 truncate text-left text-base-content/50 ml-0.5">{summary}</span>
        )}
        {block.count !== undefined && block.count > 0 && (
          <span className="flex-shrink-0 text-base-content/40">· {block.count}</span>
        )}
      </button>

      {showDetail && (block.error || block.refId) && (
        <div className="mt-2 ml-2 pl-3 border-l-2 border-base-300 text-xs text-base-content/60 space-y-1">
          {block.error && (
            <>
              <div>
                {block.error.code}: {block.error.message}
              </div>
              {block.error.hint && <div className="text-base-content/40">{block.error.hint}</div>}
            </>
          )}
          {block.refId && (
            <div className="text-base-content/40">
              {t("lumilio.tools.resultSet", "Result set")} · {block.count}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
