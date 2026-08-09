import { useI18n } from "@/lib/i18n.tsx";

export function StorageRiskConfirmation({
  warnings,
  checked,
  onChange,
  disabled = false,
}: {
  warnings: readonly string[];
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();

  if (warnings.length === 0) return null;

  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm">
      <input
        type="checkbox"
        className="checkbox checkbox-warning checkbox-sm mt-0.5"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        disabled={disabled}
      />
      <span>
        <span className="font-medium">
          {t("manage.repositories.createRiskConfirmationTitle", "Confirm storage placement risks")}
        </span>
        <span className="mt-1 block text-xs text-base-content/65">
          {t(
            "manage.repositories.createRiskConfirmationDescription",
            "This target may be removable, network-backed, or managed by cloud-sync software. Confirm that originals are backed up and available offline.",
          )}
        </span>
        <span className="mt-1 block font-mono text-[11px]">{warnings.join(" · ")}</span>
      </span>
    </label>
  );
}
