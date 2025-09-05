import { ErrorBoundary } from "react-error-boundary";
import SettingsTab from "../components/SettingsTab";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { AdjustmentsHorizontalIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

export default function Settings() {
  const { t } = useI18n();
  return (
    <ErrorBoundary
      FallbackComponent={(props) => (
        <ErrorFallBack code={500} title="Something went wrong" {...props} />
      )}
    >
      <PageHeader
        title={t("routes.settings")}
        icon={<AdjustmentsHorizontalIcon className="w-6 h-6 text-primary" />}
      />
      <SettingsTab />
    </ErrorBoundary>
  );
}
