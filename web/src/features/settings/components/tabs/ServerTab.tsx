import { useDebouncedPreference } from "../../state/preferences";
import { useRuntimeInfo } from "../../api/useRuntimeInfo";
import { useI18n } from "@/lib/i18n.tsx";
import { GaugeIcon } from "lucide-react";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../SettingsGroup";
import BackupSection from "./BackupSection";

function formatBoolean(value: boolean | undefined, t: (key: string) => string): string {
  return t(`settings.serverSettings.booleanValues.${value ? "true" : "false"}`);
}

export default function ServerTab() {
  const { t } = useI18n();
  const [healthCheckIntervalMs, setHealthCheckIntervalMs] =
    useDebouncedPreference("healthCheckIntervalMs");
  const runtimeQuery = useRuntimeInfo();
  const runtime = runtimeQuery.data;

  const valueSec = healthCheckIntervalMs / 1000;
  const presets = [1, 2, 5, 10, 30, 50];

  const setTimespan = (seconds: number) => {
    const clamped = Math.min(50, Math.max(1, seconds));
    setHealthCheckIntervalMs(clamped * 1000);
  };

  const runtimeFields: ReadonlyArray<[string, string | number | undefined]> = runtime
    ? [
        ["environment", runtime.environment],
        ["server_port", runtime.server_port],
        ["storage_root", runtime.storage_root],
        ["hardware_accel", runtime.hardware_accel],
        ["repository_scan_enabled", formatBoolean(runtime.repository_scan_enabled, t)],
        ["repository_scan_interval_seconds", runtime.repository_scan_interval_seconds],
        ["log_level", runtime.log_level],
        ["geocoding_provider", runtime.geocoding_provider],
        ["lumen_discovery_enabled", formatBoolean(runtime.lumen_discovery_enabled, t)],
      ]
    : [];

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      <SettingsGroup
        title={t("settings.serverSettings.healthCheckInterval")}
        description={t("settings.serverSettings.healthCheckDescription")}
      >
        <SettingsBlock>
          <div className="flex items-center gap-3">
            <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-success text-success-content">
              <GaugeIcon className="size-4" />
            </span>
            <input
              type="range"
              min={1}
              max={50}
              step={0.5}
              value={valueSec}
              className="range range-primary range-sm flex-1"
              onChange={(e) => setTimespan(Number(e.target.value))}
            />
            <div className="min-w-16 text-right font-mono text-sm tabular-nums">
              {valueSec.toFixed(1)}s
            </div>
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2 pl-10">
            <span className="mr-1 text-xs text-base-content/55">
              {t("settings.serverSettings.presets")}
            </span>
            {presets.map((p) => (
              <button
                key={p}
                className={`btn btn-xs ${
                  Math.abs(valueSec - p) < 0.01 ? "btn-primary" : "btn-ghost"
                }`}
                onClick={() => setTimespan(p)}
              >
                {p}s
              </button>
            ))}
            <button
              className="btn btn-xs btn-ghost text-base-content/60"
              onClick={() => setTimespan(5)}
              title="Reset to default (5s)"
            >
              {t("settings.serverSettings.reset")}
            </button>
          </div>
        </SettingsBlock>
      </SettingsGroup>

      <BackupSection />

      <SettingsGroup
        title={t("settings.serverSettings.runtimeInfoTitle", {
          defaultValue: "Runtime configuration",
        })}
        description={t("settings.serverSettings.runtimeInfoDescription", {
          defaultValue:
            "Effective server configuration. Change these values in TOML and restart the server.",
        })}
      >
        {runtimeQuery.isLoading ? (
          <SettingsBlock>
            <p className="text-sm text-base-content/60">{t("common.loading")}</p>
          </SettingsBlock>
        ) : runtimeQuery.isError || !runtime ? (
          <SettingsBlock>
            <p className="text-sm text-warning">
              {t("settings.serverSettings.runtimeInfoLoadError", {
                defaultValue: "Runtime configuration is temporarily unavailable.",
              })}
            </p>
          </SettingsBlock>
        ) : (
          runtimeFields.map(([key, displayValue]) => (
            <SettingsRow
              key={key}
              label={t(`settings.serverSettings.runtimeFields.${key}`, {
                defaultValue: key.replaceAll("_", " "),
              })}
              value={
                <span className="font-mono text-xs break-all text-base-content/70">
                  {displayValue ?? "—"}
                </span>
              }
            />
          ))
        )}
      </SettingsGroup>
    </div>
  );
}
