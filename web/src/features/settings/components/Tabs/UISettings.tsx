import React from "react";
import { LayoutDashboard } from "lucide-react";
import { useI18n, changeLanguage, getCurrentLanguage } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";

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

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-3xl font-bold my-3">
          {t("settings.ui", { defaultValue: "UI" })}
        </h2>
        <div className="flex items-center gap-3">
          <label className="font-semibold min-w-32">
            {t("settings.language", { defaultValue: "Language" })}
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
      </div>

      <h1 className="text-3xl font-bold my-3">Assets Page</h1>
      <div className="flex flex-row items-center gap-1">
        <LayoutDashboard size={20} />
        <h3 className="text-2xl font-bold my-2">Page Layout</h3>
      </div>
      <h3></h3>
    </div>
  );
}
