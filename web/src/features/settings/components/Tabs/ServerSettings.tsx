import { useWorkingRepository } from "@/features/settings/hooks/useWorkingRepository";
import { useDebouncedPreference } from "@/features/settings";
import { useRuntimeInfo } from "@/features/settings/hooks/useRuntimeInfo";
import { useI18n } from "@/lib/i18n.tsx";
import { ServerIcon } from "lucide-react";
import { SettingsSection } from "../SettingsSection";

function formatBoolean(
  value: boolean | undefined,
  t: (key: string) => string,
): string {
  return t(
    `settings.serverSettings.booleanValues.${value ? "true" : "false"}`,
  );
}

export default function ServerSettings() {
  const { t } = useI18n();
  const {
    repositories,
    repositoriesQuery,
    workingRepositoryId,
    selectedRepository,
    setWorkingRepositoryId,
    getRepositoryLabel,
  } = useWorkingRepository();
  const [healthCheckIntervalMs, setHealthCheckIntervalMs] =
    useDebouncedPreference("healthCheckIntervalMs");
  const runtimeQuery = useRuntimeInfo();
  const runtime = runtimeQuery.data?.data;

  const valueSec = healthCheckIntervalMs / 1000;

  const presets = [1, 2, 5, 10, 30, 50];

  const setTimespan = (seconds: number) => {
    const clamped = Math.min(50, Math.max(1, seconds));
    setHealthCheckIntervalMs(clamped * 1000);
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <ServerIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">{t("settings.server")}</h2>
      </div>

      <section className="space-y-2">
        <h3 className="text-lg font-semibold">
          {t("settings.serverSettings.workingRepositoryTitle", {
            defaultValue: "Working repository",
          })}
        </h3>
        <p className="text-sm opacity-70">
          {t("settings.serverSettings.workingRepositoryDescription", {
            defaultValue:
              "Choose the default repository scope for repository-aware pages and actions. Select All repositories to keep global views.",
          })}
        </p>
        <label className="form-control gap-2 max-w-xl">
          <span className="font-semibold">
            {t("settings.serverSettings.workingRepositoryLabel", {
              defaultValue: "Current application repository",
            })}
          </span>
          <select
            className="select select-bordered w-full"
            value={workingRepositoryId}
            disabled={repositoriesQuery.isLoading}
            onChange={(event) =>
              setWorkingRepositoryId(event.target.value || null)
            }
          >
            <option value="">{t("navbar.repository.all")}</option>
            {repositories.map((repository) => (
              <option key={repository.id} value={repository.id}>
                {getRepositoryLabel(repository)}
              </option>
            ))}
          </select>
          <span className="text-sm opacity-70">
            {selectedRepository?.path ??
              (repositoriesQuery.isError
                ? t("settings.serverSettings.workingRepositoryUnavailable", {
                    defaultValue:
                      "Repository options are temporarily unavailable.",
                  })
                : t("settings.serverSettings.workingRepositoryHint", {
                    defaultValue:
                      "This scope is used by assets, home, map, stats, upload, and ML indexing tools when they support repository filtering.",
                  }))}
          </span>
        </label>
      </section>

      <section className="space-y-2">
        <h3 className="text-lg font-semibold">
          {t("settings.serverSettings.healthCheckInterval")}
        </h3>
        <p className="text-sm opacity-70">
          {t("settings.serverSettings.healthCheckDescription")}
        </p>
        <div className="flex items-center gap-3">
          <input
            type="range"
            min={1}
            max={50}
            step={0.5}
            value={valueSec}
            className="range range-primary"
            onChange={(e) => setTimespan(Number(e.target.value))}
          />
          <div className="min-w-24 text-right font-mono tabular-nums">
            {valueSec.toFixed(1)}s
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm opacity-70 mr-1">
            {t("settings.serverSettings.presets")}
          </span>
          {presets.map((p) => (
            <button
              key={p}
              className={`btn btn-xs sm:btn-sm ${Math.abs(valueSec - p) < 0.01 ? "btn-primary" : "btn-outline"}`}
              onClick={() => setTimespan(p)}
            >
              {p}s
            </button>
          ))}
          <div className="divider divider-horizontal mx-1" />
          <button
            className="btn btn-xs sm:btn-sm btn-ghost"
            onClick={() => setTimespan(5)}
            title="Reset to default (5s)"
          >
            {t("settings.serverSettings.reset")}
          </button>
        </div>
      </section>

      <SettingsSection
        title={t("settings.serverSettings.runtimeInfoTitle", {
          defaultValue: "Runtime configuration",
        })}
        description={t("settings.serverSettings.runtimeInfoDescription", {
          defaultValue:
            "Effective server configuration. Change these values in TOML and restart the server.",
        })}
        variant="readonly"
      >
        {runtimeQuery.isLoading ? (
          <p className="text-sm text-base-content/70">{t("common.loading")}</p>
        ) : runtimeQuery.isError || !runtime ? (
          <div className="alert alert-warning">
            <span>
              {t("settings.serverSettings.runtimeInfoLoadError", {
                defaultValue: "Runtime configuration is temporarily unavailable.",
              })}
            </span>
          </div>
        ) : (
          <dl className="grid gap-3 sm:grid-cols-2">
            {(
              [
                ["environment", runtime.environment],
                ["server_port", runtime.server_port],
                ["storage_root", runtime.storage_root],
                ["hardware_accel", runtime.hardware_accel],
                [
                  "repository_scan_enabled",
                  formatBoolean(runtime.repository_scan_enabled, t),
                ],
                [
                  "repository_scan_interval_seconds",
                  runtime.repository_scan_interval_seconds,
                ],
                ["log_level", runtime.log_level],
                ["geocoding_provider", runtime.geocoding_provider],
                [
                  "lumen_discovery_enabled",
                  formatBoolean(runtime.lumen_discovery_enabled, t),
                ],
              ] as const
            ).map(([key, displayValue]) => (
              <div key={key} className="rounded-lg border border-base-300 px-3 py-2">
                <dt className="text-xs font-medium text-base-content/60">
                  {t(`settings.serverSettings.runtimeFields.${key}`, {
                    defaultValue: key.replaceAll("_", " "),
                  })}
                </dt>
                <dd className="mt-1 font-mono text-sm break-all">
                  {displayValue ?? "—"}
                </dd>
              </div>
            ))}
          </dl>
        )}
      </SettingsSection>
    </div>
  );
}
