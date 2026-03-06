import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import {
  PhotoIcon,
  GlobeAltIcon,
  PaintBrushIcon,
} from "@heroicons/react/24/outline";

export default function UISettings() {
  const { t } = useI18n();

  const { state, dispatch } = useSettingsContext();
  const currentLayout = state.ui.asset_page?.layout ?? "full";

  const onChangeLanguage = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const lng = e.target.value as "en" | "zh";
    dispatch({ type: "SET_LANGUAGE", payload: lng });
  };

  const onChangeRegion = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const region = e.target.value as "china" | "other";
    dispatch({ type: "SET_REGION", payload: region });
  };

  const layoutOptions = [
    {
      value: "compact" as const,
      label: t("settings.appearanceSettings.layoutOptions.compact.label"),
      description: t(
        "settings.appearanceSettings.layoutOptions.compact.description",
      ),
    },
    {
      value: "wide" as const,
      label: t("settings.appearanceSettings.layoutOptions.wide.label"),
      description: t(
        "settings.appearanceSettings.layoutOptions.wide.description",
      ),
    },
    {
      value: "full" as const,
      label: t("settings.appearanceSettings.layoutOptions.full.label"),
      description: t(
        "settings.appearanceSettings.layoutOptions.full.description",
      ),
    },
  ];

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <PaintBrushIcon className="size-6 text-primary" />
          <h2 className="text-2xl font-bold">{t("settings.appearance")}</h2>
        </div>
        <p className="text-base-content/70">
          {t("settings.appearanceSettings.description")}
        </p>
      </header>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="flex items-center gap-2">
          <GlobeAltIcon className="size-6 text-primary" />
          <h3 className="text-lg font-semibold">
            {t("settings.languageAndRegion")}
          </h3>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <label className="font-semibold block">
              {t("settings.language")}
            </label>
            <select
              className="select select-bordered w-full"
              value={state.ui.language}
              onChange={onChangeLanguage}
            >
              <option value="en">English</option>
              <option value="zh">中文</option>
            </select>
          </div>

          <div className="space-y-2">
            <label className="font-semibold block">
              {t("settings.region")}
            </label>
            <select
              className="select select-bordered w-full"
              value={state.ui.region ?? "other"}
              onChange={onChangeRegion}
            >
              <option value="china">{t("settings.regionOptions.china")}</option>
              <option value="other">{t("settings.regionOptions.other")}</option>
            </select>
          </div>
        </div>
      </section>

      <section className="rounded-2xl border border-base-300 bg-base-100 p-5 space-y-4">
        <div className="flex items-center gap-2">
          <PhotoIcon className="size-6 text-primary" />
          <div>
            <h3 className="text-lg font-semibold">{t("settings.assetPage")}</h3>
            <p className="text-sm text-base-content/70">
              {t("settings.appearanceSettings.layoutDescription")}
            </p>
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-3">
          {layoutOptions.map((option) => {
            const isActive = currentLayout === option.value;
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
                onClick={() =>
                  dispatch({
                    type: "SET_ASSETS_LAYOUT",
                    payload: option.value,
                  })
                }
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
    </div>
  );
}
