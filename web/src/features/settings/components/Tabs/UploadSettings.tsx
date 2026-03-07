import { useI18n } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import { ArrowUpTrayIcon } from "@heroicons/react/24/outline";

export default function UploadSettings() {
  const { t } = useI18n();
  const { state, dispatch } = useSettingsContext();

  const upload = {
    max_total_files: state.ui.upload?.max_total_files ?? 100,
    low_power_mode: state.ui.upload?.low_power_mode ?? true,
    chunk_size_mb: state.ui.upload?.chunk_size_mb ?? 24,
    max_concurrent_chunks: state.ui.upload?.max_concurrent_chunks ?? 2,
    use_server_config: state.ui.upload?.use_server_config ?? true,
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <ArrowUpTrayIcon className="size-6 text-primary" />
          <h2 className="text-2xl font-bold">{t("settings.upload")}</h2>
        </div>
        <p className="text-base-content/70">
          {t("settings.uploadSettings.description")}
        </p>
      </header>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
          <div className="space-y-1">
            <h3 className="text-lg font-semibold">
              {t("settings.uploadSettings.controlModeTitle")}
            </h3>
            <p className="text-sm text-base-content/70">
              {t("settings.uploadSettings.controlModeDescription")}
            </p>
          </div>

          <label className="flex items-center gap-3">
            <span className="text-sm font-medium">
              {t("settings.uploadSettings.serverMode")}
            </span>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={upload.use_server_config}
              onChange={(e) =>
                dispatch({
                  type: "SET_UPLOAD_USE_SERVER_CONFIG",
                  payload: e.target.checked,
                })
              }
            />
          </label>
        </div>

        <div className="rounded-2xl bg-base-200/70 p-4">
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={[
                "badge",
                upload.use_server_config ? "badge-primary" : "badge-ghost",
              ].join(" ")}
            >
              {upload.use_server_config
                ? t("settings.uploadSettings.serverMode")
                : t("settings.uploadSettings.localMode")}
            </span>
            {upload.use_server_config && (
              <span className="badge badge-outline">
                {t("settings.uploadSettings.recommended")}
              </span>
            )}
          </div>
          <p className="mt-2 text-sm text-base-content/70">
            {upload.use_server_config
              ? t("settings.uploadSettings.serverModeHint")
              : t("settings.uploadSettings.localModeHint")}
          </p>
        </div>
      </section>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="space-y-1">
          <h3 className="text-lg font-semibold">
            {t("settings.uploadSettings.basicTitle")}
          </h3>
          <p className="text-sm text-base-content/70">
            {t("settings.uploadSettings.basicDescription")}
          </p>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-3">
            <div>
              <div className="font-semibold">
                {t("settings.uploadSettings.maxTotalFiles")}
              </div>
              <p className="mt-1 text-sm text-base-content/70">
                {t("settings.uploadSettings.maxTotalFilesDescription")}
              </p>
            </div>
            <input
              type="number"
              min="1"
              max="500"
              className="input input-bordered w-full"
              value={upload.max_total_files}
              onChange={(e) => {
                const value = parseInt(e.target.value) || 1;
                dispatch({
                  type: "SET_UPLOAD_MAX_TOTAL_FILES",
                  payload: value,
                });
              }}
            />
          </div>

          <div className="rounded-2xl border border-base-300 bg-base-100 p-4 flex items-start justify-between gap-4">
            <div>
              <div className="font-semibold">
                {t("settings.uploadSettings.lowPowerMode")}
              </div>
              <p className="mt-1 text-sm text-base-content/70">
                {t("settings.uploadSettings.lowPowerModeDescription")}
              </p>
            </div>
            <input
              type="checkbox"
              className="toggle toggle-primary"
              checked={upload.low_power_mode}
              onChange={(e) =>
                dispatch({
                  type: "SET_UPLOAD_LOW_POWER_MODE",
                  payload: e.target.checked,
                })
              }
            />
          </div>
        </div>
      </section>

      <div className="collapse collapse-arrow rounded-2xl border border-base-300 bg-base-100">
        <input
          type="checkbox"
          aria-label={t("settings.uploadSettings.advancedTitle")}
          defaultChecked={!upload.use_server_config}
        />
        <div className="collapse-title">
          <div className="font-semibold">
            {t("settings.uploadSettings.advancedTitle")}
          </div>
          <div className="text-sm text-base-content/70">
            {t("settings.uploadSettings.advancedDescription")}
          </div>
        </div>
        <div className="collapse-content space-y-4">
          {upload.use_server_config && (
            <div className="alert alert-info">
              <span>{t("settings.uploadSettings.advancedDisabledHint")}</span>
            </div>
          )}

          <div className="grid gap-4 md:grid-cols-2">
            <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-3">
              <div>
                <div className="font-semibold">
                  {t("settings.uploadSettings.chunkSize")}
                </div>
                <p className="mt-1 text-sm text-base-content/70">
                  {t("settings.uploadSettings.chunkSizeDescription")}
                </p>
              </div>
              <div className="join w-full">
                <input
                  type="number"
                  min="1"
                  max="128"
                  disabled={upload.use_server_config}
                  className="input input-bordered join-item w-full"
                  value={upload.chunk_size_mb}
                  onChange={(e) => {
                    const value = parseInt(e.target.value) || 1;
                    dispatch({
                      type: "SET_UPLOAD_CHUNK_SIZE_MB",
                      payload: value,
                    });
                  }}
                />
                <span className="join-item btn btn-disabled">
                  {t("settings.uploadSettings.chunkSizeUnit")}
                </span>
              </div>
            </div>

            <div className="rounded-2xl border border-base-300 bg-base-100 p-4 space-y-3">
              <div>
                <div className="font-semibold">
                  {t("settings.uploadSettings.maxConcurrentChunks")}
                </div>
                <p className="mt-1 text-sm text-base-content/70">
                  {t("settings.uploadSettings.maxConcurrentChunksDescription")}
                </p>
              </div>
              <input
                type="number"
                min="1"
                max="6"
                disabled={upload.use_server_config}
                className="input input-bordered w-full"
                value={upload.max_concurrent_chunks}
                onChange={(e) => {
                  const value = parseInt(e.target.value) || 1;
                  dispatch({
                    type: "SET_UPLOAD_MAX_CONCURRENT_CHUNKS",
                    payload: value,
                  });
                }}
              />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
