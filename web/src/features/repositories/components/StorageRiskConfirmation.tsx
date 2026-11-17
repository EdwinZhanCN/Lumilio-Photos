import { useI18n } from "@/lib/i18n.tsx";
import { storageRiskLabel } from "../model/storageRiskCopy";

/**
 * The shared risk review: the risks the Server's assessment reported for this
 * exact target, acknowledged for this one command. The parent resets `checked`
 * whenever the assessment token changes, because an acknowledgement never
 * outlives the facts it was given for.
 */
export function StorageRiskConfirmation({
  risks,
  checked,
  onChange,
  disabled = false,
}: {
  risks: readonly string[];
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}) {
  const { t } = useI18n();

  if (risks.length === 0) return null;

  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-warning/40 bg-warning/10 p-3 text-sm">
      <input
        type="checkbox"
        className="checkbox checkbox-warning checkbox-sm mt-0.5"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        disabled={disabled}
      />
      <span className="min-w-0">
        <span className="font-medium">
          {t("manage.repositories.createRiskConfirmationTitle", "Acknowledge storage risks")}
        </span>
        <span className="mt-1 block text-xs text-base-content/65">
          {t(
            "manage.repositories.createRiskConfirmationDescription",
            "Lumilio reviewed this exact target. Acknowledge the risks below for this action only; a changed disk or folder asks again.",
          )}
        </span>
        <ul className="mt-2 list-disc space-y-1 pl-4 text-xs">
          {risks.map((risk) => (
            <li key={risk}>{storageRiskLabel(risk, t)}</li>
          ))}
        </ul>
      </span>
    </label>
  );
}
