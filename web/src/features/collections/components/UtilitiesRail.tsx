import { Copy } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";

/**
 * UtilitiesRail surfaces fixed maintenance shortcuts at the top of the
 * Collections page. For now it only exposes the Duplicates review entry,
 * matching the lightweight fixed-entry style used by MapRail.
 */
export default function UtilitiesRail() {
  const { t } = useI18n();
  const navigate = useNavigate();

  return (
    <div className="flex gap-4 overflow-x-auto pb-2">
      <button
        type="button"
        onClick={() => navigate("/collections/utilities/duplicates")}
        className="group w-48 shrink-0 cursor-pointer"
      >
        <div className="relative aspect-square overflow-hidden rounded-[1.75rem] bg-gradient-to-br from-primary/20 via-primary/10 to-base-200 transition duration-300">
          <div className="flex h-full flex-col items-center justify-center gap-2">
            <Copy className="size-10 text-primary" strokeWidth={1.5} />
            <span className="text-sm font-semibold text-primary">
              {t("collections.utilities.duplicates.title")}
            </span>
          </div>
          <div className="absolute inset-x-0 bottom-0 p-3">
            <p className="text-xs text-base-content/50">
              {t("collections.utilities.duplicates.description")}
            </p>
          </div>
        </div>
      </button>
    </div>
  );
}
