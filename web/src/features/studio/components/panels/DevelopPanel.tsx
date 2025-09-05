import { AdjustmentsHorizontalIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

export function DevelopPanel() {
  const { t } = useI18n();
  return (
    <div className="h-full flex items-center justify-center text-center p-4 rounded-lg bg-base-100 min-h-[400px]">
      <div>
        <AdjustmentsHorizontalIcon className="w-12 h-12 mx-auto text-base-content/50" />
        <h3 className="mt-2 text-lg font-semibold">
          {t("studio.develop.title")}
        </h3>
        <p className="mt-1 text-sm text-base-content/70">
          {t("studio.develop.desc")}
        </p>
      </div>
    </div>
  );
}
