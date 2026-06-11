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

  return (
    <div className="my-2">
      <button
        onClick={() => setShowDetail(!showDetail)}
        className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full border text-xs font-medium transition-colors duration-200 cursor-pointer ${accent}`}
      >
        <Hammer className="flex-shrink-0" size={12} strokeWidth={1.5} />
        <span className="truncate max-w-[160px]">{block.name}</span>
        {block.status === "running" && (
          <Loader className="animate-spin" size={13} />
        )}
        {block.status === "success" &&
          (isEmptyResult ? (
            <TriangleAlert size={13} />
          ) : (
            <BadgeCheck size={13} />
          ))}
        {block.status === "error" && <X size={13} />}
        <span className="text-base-content/50 truncate max-w-[260px] ml-0.5">
          {isEmptyResult
            ? t("lumilio.tools.emptyResult", "No matching photos")
            : (block.message ?? block.error?.message)}
        </span>
        {block.count !== undefined && block.count > 0 && (
          <span className="text-base-content/40">· {block.count}</span>
        )}
      </button>

      {showDetail && (block.error || block.refId) && (
        <div className="mt-2 ml-2 pl-3 border-l-2 border-base-300 text-xs text-base-content/60 space-y-1">
          {block.error && (
            <>
              <div>
                {block.error.code}: {block.error.message}
              </div>
              {block.error.hint && (
                <div className="text-base-content/40">{block.error.hint}</div>
              )}
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
