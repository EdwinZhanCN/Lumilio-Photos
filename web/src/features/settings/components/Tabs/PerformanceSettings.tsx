import { useMemo } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  usePerformancePreferences,
  PerformanceProfile,
} from "@/lib/utils/performancePreferences.ts";
import { detectDeviceCapabilities } from "@/lib/utils/smartBatchSizing.ts";

export default function PerformanceSettings() {
  const { t } = useI18n();
  const { preferences, updatePreferences, resetToDefaults } =
    usePerformancePreferences();
  const deviceInfo = useMemo(() => detectDeviceCapabilities(), []);

  const handleProfileChange = (profile: PerformanceProfile) => {
    updatePreferences({ profile });
  };

  const handleToggleChange = (
    key: "respectMemoryLimits" | "prioritizeUserOperations",
    value: boolean,
  ) => {
    updatePreferences({ [key]: value });
  };

  const handleNumberChange = (
    key: "maxConcurrentOperations",
    value: number,
  ) => {
    updatePreferences({ [key]: value });
  };

  const handleReset = () => {
    resetToDefaults();
  };

  const profileOptions = [
    {
      value: PerformanceProfile.MEMORY_SAVER,
      label: t("settings.performanceSettings.profiles.memory_saver.label"),
      description: t(
        "settings.performanceSettings.profiles.memory_saver.description",
      ),
    },
    {
      value: PerformanceProfile.BALANCED,
      label: t("settings.performanceSettings.profiles.balanced.label"),
      description: t(
        "settings.performanceSettings.profiles.balanced.description",
      ),
    },
    {
      value: PerformanceProfile.SPEED_OPTIMIZED,
      label: t("settings.performanceSettings.profiles.speed.label"),
      description: t("settings.performanceSettings.profiles.speed.description"),
    },
    {
      value: PerformanceProfile.ADAPTIVE,
      label: t("settings.performanceSettings.profiles.adaptive.label"),
      description: t(
        "settings.performanceSettings.profiles.adaptive.description",
      ),
    },
  ];

  const currentProfile =
    profileOptions.find((option) => option.value === preferences.profile) ??
    profileOptions[0];

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="space-y-2">
          <h2 className="text-2xl font-bold">{t("settings.performance")}</h2>
          <p className="text-base-content/70 max-w-2xl">
            {t("settings.performanceSettings.description")}
          </p>
        </div>

        <button className="btn btn-outline btn-sm" onClick={handleReset}>
          {t("settings.performanceSettings.reset")}
        </button>
      </header>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="flex flex-wrap items-center gap-2">
          <span className="badge badge-primary badge-outline">
            {t("settings.performanceSettings.currentProfile")}:{" "}
            {currentProfile.label}
          </span>
          <span className="badge badge-ghost">
            {t("settings.performanceSettings.cpuCores")}: {deviceInfo.cpuCores}
          </span>
          <span className="badge badge-ghost">
            {t("settings.performanceSettings.availableMemory")}: ~
            {deviceInfo.availableMemoryMB}MB
          </span>
          <span className="badge badge-ghost">
            {t("settings.performanceSettings.deviceType")}:{" "}
            {deviceInfo.isMobile
              ? t("settings.performanceSettings.deviceTypeValues.mobile")
              : t("settings.performanceSettings.deviceTypeValues.desktop")}
            {deviceInfo.isLowEndDevice
              ? ` · ${t("settings.performanceSettings.deviceTypeValues.lowEnd")}`
              : ""}
          </span>
          <span className="badge badge-ghost">
            {t("settings.performanceSettings.maxConcurrency")}:{" "}
            {deviceInfo.maxConcurrency}
          </span>
        </div>

        <p className="text-sm text-base-content/70">
          {currentProfile.description}
        </p>
      </section>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="space-y-1">
          <h3 className="text-lg font-semibold">
            {t("settings.performanceSettings.profileTitle")}
          </h3>
          <p className="text-sm text-base-content/70">
            {t("settings.performanceSettings.profileDescription")}
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          {profileOptions.map((option) => {
            const isActive = preferences.profile === option.value;
            return (
              <button
                key={option.value}
                type="button"
                className={[
                  "rounded-2xl border p-4 text-left transition",
                  isActive
                    ? "border-primary bg-primary/8 shadow-sm"
                    : "border-base-300 bg-base-100 hover:border-base-content/30",
                ].join(" ")}
                onClick={() => handleProfileChange(option.value)}
              >
                <div className="font-semibold">{option.label}</div>
                <div className="mt-2 text-sm text-base-content/70">
                  {option.description}
                </div>
              </button>
            );
          })}
        </div>
      </section>

      <div className="collapse collapse-arrow rounded-2xl border border-base-300 bg-base-100">
        <input
          type="checkbox"
          aria-label={t("settings.performanceSettings.advancedTitle")}
        />
        <div className="collapse-title">
          <div className="font-semibold">
            {t("settings.performanceSettings.advancedTitle")}
          </div>
          <div className="text-sm text-base-content/70">
            {t("settings.performanceSettings.advancedDescription")}
          </div>
        </div>
        <div className="collapse-content space-y-4">
          <div className="rounded-2xl border border-base-300 bg-base-100 p-4 flex items-start justify-between gap-4">
            <div>
              <div className="font-semibold">
                {t("settings.performanceSettings.respectMemoryLimits")}
              </div>
              <div className="mt-1 text-sm text-base-content/70">
                {t(
                  "settings.performanceSettings.respectMemoryLimitsDescription",
                )}
              </div>
            </div>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={preferences.respectMemoryLimits}
              onChange={(e) =>
                handleToggleChange("respectMemoryLimits", e.target.checked)
              }
            />
          </div>

          <div className="rounded-2xl border border-base-300 bg-base-100 p-4 flex items-start justify-between gap-4">
            <div>
              <div className="font-semibold">
                {t("settings.performanceSettings.prioritizeUserOperations")}
              </div>
              <div className="mt-1 text-sm text-base-content/70">
                {t(
                  "settings.performanceSettings.prioritizeUserOperationsDescription",
                )}
              </div>
            </div>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={preferences.prioritizeUserOperations}
              onChange={(e) =>
                handleToggleChange("prioritizeUserOperations", e.target.checked)
              }
            />
          </div>

          <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-3">
            <div>
              <div className="font-semibold">
                {t("settings.performanceSettings.maxConcurrentOperations")}
              </div>
              <div className="mt-1 text-sm text-base-content/70">
                {t(
                  "settings.performanceSettings.maxConcurrentOperationsDescription",
                )}
              </div>
            </div>
            <div className="flex items-center gap-4">
              <input
                type="range"
                min={1}
                max={8}
                value={preferences.maxConcurrentOperations}
                onChange={(e) =>
                  handleNumberChange(
                    "maxConcurrentOperations",
                    parseInt(e.target.value),
                  )
                }
                className="range range-primary flex-1"
              />
              <span className="w-8 text-center font-mono">
                {preferences.maxConcurrentOperations}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="alert alert-info">
        <span>{t("settings.performanceSettings.changesApply")}</span>
      </div>
    </div>
  );
}
