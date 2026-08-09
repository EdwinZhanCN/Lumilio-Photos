import { useI18n } from "@/lib/i18n.tsx";

export type RepositoryStorageStrategy = "date" | "flat" | "cas";

const STRATEGIES: readonly RepositoryStorageStrategy[] = ["date", "flat", "cas"];

export function StorageStrategyPicker({
  value,
  onChange,
  disabled = false,
  idPrefix = "repository",
}: {
  value: RepositoryStorageStrategy;
  onChange: (strategy: RepositoryStorageStrategy) => void;
  disabled?: boolean;
  idPrefix?: string;
}) {
  const { t } = useI18n();

  return (
    <fieldset className="fieldset gap-2">
      <legend className="fieldset-legend p-0 text-sm font-medium">
        {t("manage.repositories.storageStrategy.label", "Storage layout")}
      </legend>
      <div className="grid gap-2 sm:grid-cols-3">
        {STRATEGIES.map((strategy) => {
          const id = `${idPrefix}-storage-strategy-${strategy}`;
          return (
            <label
              key={strategy}
              htmlFor={id}
              className={`cursor-pointer rounded-lg border p-3 transition-colors ${
                value === strategy
                  ? "border-primary bg-primary/5"
                  : "border-base-300 hover:border-base-content/30"
              } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
            >
              <span className="flex items-center gap-2 text-sm font-medium">
                <input
                  id={id}
                  type="radio"
                  className="radio radio-primary radio-xs"
                  name={`${idPrefix}-storage-strategy`}
                  value={strategy}
                  checked={value === strategy}
                  disabled={disabled}
                  onChange={() => onChange(strategy)}
                />
                {strategyLabel(strategy, t)}
              </span>
              <span className="mt-1 block text-xs leading-snug text-base-content/60">
                {strategyDescription(strategy, t)}
              </span>
            </label>
          );
        })}
      </div>
      <span className="label text-xs leading-snug text-warning">
        {t(
          "manage.repositories.storageStrategy.immutableHint",
          "This choice determines the on-disk layout and cannot be changed after creation.",
        )}
      </span>
    </fieldset>
  );
}

function strategyLabel(strategy: RepositoryStorageStrategy, t: ReturnType<typeof useI18n>["t"]) {
  switch (strategy) {
    case "date":
      return t("manage.repositories.storageStrategy.dateLabel", "By date");
    case "flat":
      return t("manage.repositories.storageStrategy.flatLabel", "Single folder");
    case "cas":
      return t("manage.repositories.storageStrategy.casLabel", "Content addressed");
  }
}

function strategyDescription(
  strategy: RepositoryStorageStrategy,
  t: ReturnType<typeof useI18n>["t"],
) {
  switch (strategy) {
    case "date":
      return t(
        "manage.repositories.storageStrategy.dateDescription",
        "Organizes imports as inbox/YYYY/MM/photo.jpg using the UTC import time.",
      );
    case "flat":
      return t(
        "manage.repositories.storageStrategy.flatDescription",
        "Keeps imported files together in one directory.",
      );
    case "cas":
      return t(
        "manage.repositories.storageStrategy.casDescription",
        "Uses content hashes for deterministic file paths.",
      );
  }
}
