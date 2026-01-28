import { useI18n } from "@/lib/i18n";

/**
 * Props for the ProgressIndicator component.
 */
type ProgressIndicatorProps = {
  /** Number of items that have been processed */
  processed?: number;
  /** Total number of items to process */
  total?: number;
  /** Optional label to display alongside the progress */
  label?: string;
};

/**
 * A progress indicator component for showing upload/processing progress.
 * 
 * This component displays a visual progress bar along with text showing
 * the current progress in the format "processed/total label". It supports
 * internationalization and is commonly used for file upload operations.
 * 
 * @param props - Component props
 * @param props.processed - Number of items that have been processed
 * @param props.total - Total number of items to process
 * @param props.label - Optional label to display alongside the progress
 * 
 * @example
 * ```typescript
 * // Basic usage
 * <ProgressIndicator processed={25} total={100} />
 * 
 * // With custom label
 * <ProgressIndicator 
 *   processed={5} 
 *   total={10} 
 *   label="files uploaded" 
 * />
 * 
 * // For hash generation
 * <ProgressIndicator 
 *   processed={3} 
 *   total={7} 
 *   label="hashes generated" 
 * />
 * ```
 */
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
