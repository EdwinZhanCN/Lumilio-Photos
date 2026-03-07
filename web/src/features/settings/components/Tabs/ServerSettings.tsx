import { ServerStackIcon } from "@heroicons/react/24/outline";
import { useSettingsContext } from "@/features/settings";
import { useWorkingRepository } from "@/features/settings/hooks/useWorkingRepository";
import { useI18n } from "@/lib/i18n.tsx";

export default function ServerSettings() {
  const { t } = useI18n();
  const { state, dispatch } = useSettingsContext();
  const {
    repositories,
    repositoriesQuery,
    workingRepositoryId,
    selectedRepository,
    setWorkingRepositoryId,
    getRepositoryLabel,
  } = useWorkingRepository();
  const value = state.server.update_timespan;

  // Reasonable presets within [1, 50] seconds
  const presets = [1, 2, 5, 10, 30, 50];

  const setTimespan = (v: number) => {
    const clamped = Math.min(50, Math.max(1, v));
    dispatch({ type: "SET_SERVER_UPDATE_TIMESPAN", payload: clamped });
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <ServerStackIcon className="size-6 text-primary" />
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
            value={value}
            className="range range-primary"
            onChange={(e) => setTimespan(Number(e.target.value))}
          />
          <div className="min-w-24 text-right font-mono tabular-nums">
            {value.toFixed(1)}s
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm opacity-70 mr-1">
            {t("settings.serverSettings.presets")}
          </span>
          {presets.map((p) => (
            <button
              key={p}
              className={`btn btn-xs sm:btn-sm ${value === p ? "btn-primary" : "btn-outline"}`}
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
    </div>
  );
}
