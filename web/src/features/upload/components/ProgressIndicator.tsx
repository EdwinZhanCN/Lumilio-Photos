import { useI18n } from "@/lib/i18n";

type ProgressIndicatorProps = {
  processed?: number;
  total?: number;
  label?: string;
};

const ProgressIndicator = ({
  processed,
  total,
  label,
}: ProgressIndicatorProps) => {
  const { t } = useI18n();
  return (
    <div className="mb-4">
      <div className="flex items-center gap-2">
        <progress
          className="progress w-56"
          value={processed}
          max={total}
        ></progress>
        <span className="text-sm text-gray-500">
          {t('upload.ProgressIndicator.progress_display', { processed, total, label })}
        </span>
      </div>
    </div>
  );
};

export default ProgressIndicator;
