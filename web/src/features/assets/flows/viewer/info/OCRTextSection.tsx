import { useEffect, useMemo, useState } from "react";
import { Check, Copy, RefreshCw, ScanText } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { copyText } from "@/lib/clipboard";
import { useI18n } from "@/lib/i18n.tsx";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import type { components } from "@/lib/http-commons/schema.d.ts";

type OCRResult = components["schemas"]["dto.AssetOCRResultDTO"];

interface OCRTextSectionProps {
  result?: OCRResult;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
}

export function OCRTextSection({ result, isLoading, error, onRetry }: OCRTextSectionProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [copied, setCopied] = useState(false);
  const lines = useMemo(
    () =>
      (result?.text_items ?? [])
        .map((item) => item.text_content ?? "")
        .filter((line) => line.trim().length > 0),
    [result?.text_items],
  );
  const copyValue = lines.join("\n");
  const regionCount = result?.total_count ?? result?.text_items?.length ?? 0;

  useEffect(() => {
    setCopied(false);
  }, [copyValue]);

  const handleCopy = async () => {
    if (!copyValue) return;
    try {
      await copyText(copyValue);
      setCopied(true);
    } catch {
      setCopied(false);
      showMessage("error", t("common.copyFailed", { defaultValue: "Copy failed." }));
    }
  };

  return (
    <section className="rounded bg-base-200 p-3" aria-labelledby="asset-ocr-title">
      <div className="mb-2 flex min-h-6 items-center justify-between gap-2">
        <div className="flex min-w-0 items-center gap-1.5">
          <ScanText className="size-3.5 shrink-0 text-base-content" aria-hidden="true" />
          <h2
            id="asset-ocr-title"
            className="truncate text-[10px] font-bold uppercase tracking-wider text-base-content/50"
          >
            {t("assets.ocr.title", "OCR Text Recognition")}
          </h2>
          {result && (
            <span className="text-[10px] text-base-content/40">
              {t("assets.ocr.regions", {
                count: regionCount,
                defaultValue: "{{count}} text regions",
              })}
            </span>
          )}
        </div>
        {copyValue && (
          <button type="button" className="btn btn-ghost btn-xs shrink-0" onClick={handleCopy}>
            {copied ? (
              <Check className="size-3.5" aria-hidden="true" />
            ) : (
              <Copy className="size-3.5" aria-hidden="true" />
            )}
            {copied
              ? t("common.copied", { defaultValue: "Copied" })
              : t("common.copy", { defaultValue: "Copy" })}
          </button>
        )}
      </div>

      {isLoading ? (
        <div
          role="status"
          aria-label={t("assets.ocr.loading", "Loading OCR Text Recognition result")}
          className="space-y-2 py-1"
        >
          <div className="skeleton h-3 w-full" />
          <div className="skeleton h-3 w-4/5" />
        </div>
      ) : error ? (
        <div role="alert" className="flex items-start justify-between gap-3 text-xs">
          <span className="text-error">
            {localizeAPIProblem(
              error,
              t,
              t("assets.ocr.loadFailed", "OCR Text Recognition result could not be loaded."),
            )}
          </span>
          <button type="button" className="btn btn-ghost btn-xs shrink-0" onClick={onRetry}>
            <RefreshCw className="size-3.5" aria-hidden="true" />
            {t("common.retry", { defaultValue: "Retry" })}
          </button>
        </div>
      ) : !result ? (
        <p className="text-xs text-base-content/50">
          {t("assets.ocr.unavailable", "No stored OCR Text Recognition result is available.")}
        </p>
      ) : lines.length === 0 ? (
        <p className="text-xs text-base-content/50">
          {t("assets.ocr.empty", "No text was recognized.")}
        </p>
      ) : (
        <ol className="space-y-1.5 font-sans text-sm leading-relaxed">
          {lines.map((line, index) => (
            <li key={`${index}:${line}`} className="whitespace-pre-wrap break-words">
              {line}
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
