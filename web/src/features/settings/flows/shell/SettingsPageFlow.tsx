import { ErrorBoundary } from "react-error-boundary";
import SettingsShell from "./SettingsShell";
import ErrorFallback from "@/components/ui/ErrorFallback";
import PageHeader from "@/components/ui/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { SlidersHorizontalIcon } from "lucide-react";

export default function Settings() {
  const { t } = useI18n();

  return (
    <div className="flex h-full min-h-0 flex-col">
      <ErrorBoundary
        FallbackComponent={(props) => (
          <ErrorFallback code={500} title="Something went wrong" {...props} />
        )}
      >
        <PageHeader
          title={t("routes.settings")}
          icon={<SlidersHorizontalIcon className="w-6 h-6 text-primary" />}
        />
        <div className="min-h-0 flex-1 overflow-y-auto overflow-x-hidden">
          <SettingsShell />
        </div>
      </ErrorBoundary>
    </div>
  );
}
