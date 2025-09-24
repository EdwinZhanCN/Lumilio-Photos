import React from "react";
import { LayoutDashboard } from "lucide-react";
import { useI18n, changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import {
  PhotoIcon,
  GlobeAltIcon,
  CursorArrowRippleIcon,
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
    </div>
  );
}
