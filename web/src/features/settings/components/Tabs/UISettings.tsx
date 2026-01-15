import React from "react";
import { LayoutDashboard } from "lucide-react";
import { useI18n, changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import {
  PhotoIcon,
  GlobeAltIcon,
  CursorArrowRippleIcon,
  ArrowUpTrayIcon,
} from "@heroicons/react/24/outline";

export default function UISettings() {
  const { t } = useI18n();

  const { state, dispatch } = useSettingsContext();

  React.useEffect(() => {
    const lng = state.ui.language ?? getCurrentLanguage();
    document.documentElement.setAttribute("lang", lng);
    changeLanguage(lng);
  }, [state.ui.language]);

  const onChangeLanguage = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const lng = e.target.value as "en" | "zh";
    dispatch({ type: "SET_LANGUAGE", payload: lng });
    document.documentElement.setAttribute("lang", lng);
    changeLanguage(lng);
  };

  const onChangeRegion = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const region = e.target.value as "china" | "other";
    dispatch({ type: "SET_REGION", payload: region });
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <CursorArrowRippleIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">{t("settings.ui")}</h2>
      </div>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <GlobeAltIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">
            {t("settings.languageAndRegion")}
          </h3>
        </div>

        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-32">
              {t("settings.language")}
            </label>
            <select
              className="select select-bordered"
              value={state.ui.language ?? getCurrentLanguage()}
              onChange={onChangeLanguage}
            >
              <option value="en">English</option>
              <option value="zh">中文</option>
            </select>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-32">
              {t("settings.region")}
            </label>
            <select
              className="select select-bordered"
              value={state.ui.region ?? "other"}
              onChange={onChangeRegion}
            >
              <option value="china">{t("settings.regionOptions.china")}</option>
              <option value="other">{t("settings.regionOptions.other")}</option>
            </select>
          </div>
        </div>
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <PhotoIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">{t("settings.assetPage")}</h3>
        </div>

        <div className="flex items-center gap-2">
          <LayoutDashboard className="size-6 text-primary" />
          <h4 className="text-base font-medium">{t("settings.pageLayout")}</h4>
        </div>
      </section>

      <section className="space-y-4">
        <div className="flex items-center gap-2">
          <ArrowUpTrayIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">Upload Settings</h3>
        </div>

        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">Max Preview Count</label>
            <input
              type="number"
              className="input input-bordered w-32"
              min="0"
              max="100"
              value={state.ui.upload?.max_preview_count ?? 30}
              onChange={(e) => {
                const value = parseInt(e.target.value) || 0;
                dispatch({
                  type: "SET_UPLOAD_MAX_PREVIEW_COUNT",
                  payload: value,
                });
              }}
            />
            <span className="text-sm text-base-content/70">
              Number of files to generate thumbnails for
            </span>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">Max Total Files</label>
            <input
              type="number"
              className="input input-bordered w-32"
              min="1"
              max="500"
              value={state.ui.upload?.max_total_files ?? 100}
              onChange={(e) => {
                const value = parseInt(e.target.value) || 1;
                dispatch({
                  type: "SET_UPLOAD_MAX_TOTAL_FILES",
                  payload: value,
                });
              }}
            />
            <span className="text-sm text-base-content/70">
              Maximum number of files that can be uploaded at once
            </span>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">Low Power Mode</label>
            <input
              type="checkbox"
              className="toggle"
              checked={state.ui.upload?.low_power_mode ?? false}
              onChange={(e) =>
                dispatch({
                  type: "SET_UPLOAD_LOW_POWER_MODE",
                  payload: e.target.checked,
                })
              }
            />
            <span className="text-sm text-base-content/70">
              Reduce CPU by larger chunks + lower concurrency
            </span>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">Chunk Size (MB)</label>
            <input
              type="number"
              className="input input-bordered w-32"
              min="1"
              max="128"
              value={state.ui.upload?.chunk_size_mb ?? 24}
              onChange={(e) => {
                const value = parseInt(e.target.value) || 1;
                dispatch({
                  type: "SET_UPLOAD_CHUNK_SIZE_MB",
                  payload: value,
                });
              }}
            />
            <span className="text-sm text-base-content/70">
              Default 24 MB when low power mode is on
            </span>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">
              Max Concurrent Chunks
            </label>
            <input
              type="number"
              className="input input-bordered w-32"
              min="1"
              max="6"
              value={state.ui.upload?.max_concurrent_chunks ?? 2}
              onChange={(e) => {
                const value = parseInt(e.target.value) || 1;
                dispatch({
                  type: "SET_UPLOAD_MAX_CONCURRENT_CHUNKS",
                  payload: value,
                });
              }}
            />
            <span className="text-sm text-base-content/70">
              Concurrent uploads per file
            </span>
          </div>

          <div className="flex items-center gap-3">
            <label className="font-semibold min-w-40">
              Use Server Upload Config
            </label>
            <input
              type="checkbox"
              className="toggle"
              checked={state.ui.upload?.use_server_config ?? true}
              onChange={(e) =>
                dispatch({
                  type: "SET_UPLOAD_USE_SERVER_CONFIG",
                  payload: e.target.checked,
                })
              }
            />
            <span className="text-sm text-base-content/70">
              Auto-apply backend chunk size/concurrency hints
            </span>
          </div>
        </div>
      </section>
    </div>
  );
}
