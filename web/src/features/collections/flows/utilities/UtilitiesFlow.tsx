import { useNavigate } from "react-router-dom";
import { ErrorBoundary } from "react-error-boundary";
import { Wrench } from "lucide-react";
import ErrorFallback from "@/components/ui/ErrorFallback";
import PageHeader from "@/components/ui/PageHeader";
import { useBreadcrumbs } from "@/components/breadcrumbs";
import { useI18n } from "@/lib/i18n.tsx";
import RailCard from "../../components/RailCard";
import { useUtilityShortcuts } from "./useUtilityShortcuts";

function UtilitiesContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const shortcuts = useUtilityShortcuts();
  useBreadcrumbs([
    { label: t("sidebar.home", "Home"), to: "/" },
    { label: t("sidebar.collections", "Collections"), to: "/collections" },
    { label: t("collections.sections.utilities", "Utilities") },
  ]);

  return (
    <div className="flex h-full flex-col">
      <PageHeader
        title={t("collections.sections.utilities", "Utilities")}
        icon={<Wrench className="h-6 w-6 text-primary" strokeWidth={1.5} />}
      />

      <div className="flex-1 min-h-0 overflow-y-auto px-4 pb-8 pt-4">
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
          {shortcuts.map((shortcut) => (
            <RailCard
              key={shortcut.key}
              media={{ kind: "icon", icon: shortcut.icon, tone: shortcut.tone }}
              title={shortcut.title}
              onClick={() => navigate(shortcut.to)}
              className="w-full"
            />
          ))}
        </div>
      </div>
    </div>
  );
}

export default function Utilities() {
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
      <UtilitiesContent />
    </ErrorBoundary>
  );
}
