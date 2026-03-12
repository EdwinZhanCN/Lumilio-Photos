import { useI18n } from "@/lib/i18n.tsx";

interface LumilioStatusProps {
  isInitializing: boolean;
  progress?: {
    initStatus?: string;
  } | null;
}

export function LumilioStatus({
  isInitializing,
  progress,
}: LumilioStatusProps) {
  const { t } = useI18n();
  if (!isInitializing || !progress || !progress.initStatus) {
    return null;
  }

  return (
    <div className="p-2 bg-info text-info-content">
      <span className="loading loading-spinner loading-sm my-2" />
      {t("lumilio.status.loading", { status: progress.initStatus })}
    </div>
  );
}
