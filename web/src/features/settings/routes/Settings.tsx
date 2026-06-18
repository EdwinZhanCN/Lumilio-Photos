import { ErrorBoundary } from "react-error-boundary";
import { SettingsShell } from "../components/renew";
import ErrorFallBack from "@/components/ErrorFallBack";
import PageHeader from "@/components/PageHeader";
import { useI18n } from "@/lib/i18n.tsx";
import { SlidersHorizontalIcon } from "lucide-react";

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
        icon={<SlidersHorizontalIcon className="w-6 h-6 text-primary" />}
      />
      <SettingsShell />
    </ErrorBoundary>
  );
}
