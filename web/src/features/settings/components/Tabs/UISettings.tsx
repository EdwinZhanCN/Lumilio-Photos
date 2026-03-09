import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import {
  MoonIcon,
  PhotoIcon,
  GlobeAltIcon,
  PaintBrushIcon,
  SunIcon,
} from "@heroicons/react/24/outline";
import {
  DAISYUI_DARK_THEMES,
  DAISYUI_LIGHT_THEMES,
  type DaisyUIDarkThemeName,
  type DaisyUILightThemeName,
} from "@/lib/theme/daisyuiThemes";

type ModeThemeName = DaisyUILightThemeName | DaisyUIDarkThemeName;

interface ThemePickerSectionProps {
  title: string;
  description: string;
  activeBadgeLabel: string;
  isActiveMode: boolean;
  themes: readonly ModeThemeName[];
  selectedTheme: ModeThemeName;
  icon: React.ComponentType<React.ComponentProps<"svg">>;
  onSelect: (theme: ModeThemeName) => void;
}

function ThemePickerSection({
  title,
  description,
  activeBadgeLabel,
  isActiveMode,
  themes,
  selectedTheme,
  icon: Icon,
  onSelect,
}: ThemePickerSectionProps) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Icon className="size-5 text-primary" />
            <h4 className="font-semibold">{title}</h4>
          </div>
          <p className="text-sm text-base-content/70">{description}</p>
        </div>
        {isActiveMode ? (
          <span className="badge badge-primary badge-soft">
            {activeBadgeLabel}
          </span>
        ) : null}
      </div>

      <div className="rounded-box grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {themes.map((theme) => {
          const isSelected = selectedTheme === theme;

          return (
            <button
              key={theme}
              type="button"
              aria-pressed={isSelected}
              aria-label={`${title}: ${theme}`}
              className={[
                "border-base-content/20 hover:border-base-content/40 overflow-hidden rounded-lg border outline-2 outline-offset-2 transition",
                isSelected
                  ? "border-base-content/40 outline-base-content"
                  : "outline-transparent",
              ].join(" ")}
              onClick={() => onSelect(theme)}
            >
              <div
                className="bg-base-100 text-base-content w-full cursor-pointer font-sans"
                data-theme={theme}
              >
                <div className="grid grid-cols-5 grid-rows-3">
                  <div className="bg-base-200 col-start-1 row-span-2 row-start-1" />
                  <div className="bg-base-300 col-start-1 row-start-3" />
                  <div className="bg-base-100 col-span-4 col-start-2 row-span-3 row-start-1 flex flex-col gap-1 p-2 text-left">
                    <div className="font-bold">{theme}</div>
                    <div className="flex flex-wrap gap-1">
                      <div className="bg-primary flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-primary-content text-sm font-bold">
                          A
                        </div>
                      </div>
                      <div className="bg-secondary flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-secondary-content text-sm font-bold">
                          A
                        </div>
                      </div>
                      <div className="bg-accent flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-accent-content text-sm font-bold">
                          A
                        </div>
                      </div>
                      <div className="bg-neutral flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-neutral-content text-sm font-bold">
                          A
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}

export default function UISettings() {
  const { t } = useI18n();

  const { state, dispatch, resolvedThemeMode } = useSettingsContext();
  const isFollowingSystem = state.ui.theme.followSystem;
  const currentThemeMode = resolvedThemeMode;
  const lightModeTheme = state.ui.theme.themes.light;
  const darkModeTheme = state.ui.theme.themes.dark;
  const currentLayout = state.ui.asset_page?.layout ?? "full";
  const compactColumns = state.ui.asset_page?.columns ?? 6;

  const onChangeLanguage = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const lng = e.target.value as "en" | "zh";
    dispatch({ type: "SET_LANGUAGE", payload: lng });
  };

  const onChangeRegion = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const region = e.target.value as "china" | "other";
    dispatch({ type: "SET_REGION", payload: region });
  };

  const onChangeCompactColumns = (e: React.ChangeEvent<HTMLInputElement>) => {
    dispatch({
      type: "SET_ASSETS_COLUMNS",
      payload: Number(e.target.value),
    });
  };

  const onChangeLightModeTheme = (theme: ModeThemeName) => {
    dispatch({
      type: "SET_LIGHT_MODE_THEME",
      payload: theme as DaisyUILightThemeName,
    });
  };

  const onChangeDarkModeTheme = (theme: ModeThemeName) => {
    dispatch({
      type: "SET_DARK_MODE_THEME",
      payload: theme as DaisyUIDarkThemeName,
    });
  };

  const onChangeFollowSystem = (e: React.ChangeEvent<HTMLInputElement>) => {
    const nextFollowSystem = e.target.checked;

    if (!nextFollowSystem) {
      dispatch({
        type: "SET_THEME_MODE",
        payload: resolvedThemeMode,
      });
    }

    dispatch({
      type: "SET_THEME_FOLLOW_SYSTEM",
      payload: nextFollowSystem,
    });
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
      value: "full" as const,
      label: t("settings.appearanceSettings.layoutOptions.full.label"),
      description: t(
        "settings.appearanceSettings.layoutOptions.full.description",
      ),
    },
  ];
  const compactColumnMarks = [4, 5, 6, 7, 8, 9, 10];

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
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <PaintBrushIcon className="size-6 text-primary" />
              <h3 className="text-lg font-semibold">
                {t("settings.appearanceSettings.themes.title")}
              </h3>
            </div>
            <p className="text-sm text-base-content/70">
              {t("settings.appearanceSettings.themes.description")}
            </p>
          </div>
          <div className="badge badge-primary badge-soft">
            {t("settings.appearanceSettings.themes.currentMode")}:{" "}
            {t(
              `settings.appearanceSettings.themes.modeNames.${currentThemeMode}`,
            )}
          </div>
        </div>

        <div className="flex flex-wrap items-start justify-between gap-4 rounded-2xl border border-base-300 bg-base-200/40 p-4">
          <div className="space-y-1">
            <div className="font-semibold">
              {t("settings.appearanceSettings.themes.followSystem.label")}
            </div>
            <p className="text-sm text-base-content/70">
              {t("settings.appearanceSettings.themes.followSystem.description")}
            </p>
            <p className="text-xs text-base-content/60">
              {t("settings.appearanceSettings.themes.followSystem.status", {
                mode: t(
                  `settings.appearanceSettings.themes.modeNames.${currentThemeMode}`,
                ),
              })}
            </p>
          </div>

          <input
            type="checkbox"
            className="toggle toggle-primary mt-1"
            checked={isFollowingSystem}
            onChange={onChangeFollowSystem}
          />
        </div>

        <ThemePickerSection
          title={t("settings.appearanceSettings.themes.light.label")}
          description={t(
            "settings.appearanceSettings.themes.light.description",
          )}
          activeBadgeLabel={t("settings.appearanceSettings.themes.activeBadge")}
          isActiveMode={currentThemeMode === "light"}
          themes={DAISYUI_LIGHT_THEMES}
          selectedTheme={lightModeTheme}
          icon={SunIcon}
          onSelect={onChangeLightModeTheme}
        />

        <ThemePickerSection
          title={t("settings.appearanceSettings.themes.dark.label")}
          description={t("settings.appearanceSettings.themes.dark.description")}
          activeBadgeLabel={t("settings.appearanceSettings.themes.activeBadge")}
          isActiveMode={currentThemeMode === "dark"}
          themes={DAISYUI_DARK_THEMES}
          selectedTheme={darkModeTheme}
          icon={MoonIcon}
          onSelect={onChangeDarkModeTheme}
        />
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

        <div className="grid gap-3 md:grid-cols-2">
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

        <div
          className={`rounded-2xl border border-base-300 bg-base-100 p-4 transition-opacity ${
            currentLayout === "compact" ? "" : "opacity-60"
          }`}
        >
          <div className="flex items-center justify-between gap-3">
            <div>
              <div className="font-semibold">
                {t("settings.appearanceSettings.compactColumns.label", {
                  defaultValue: "Compact columns",
                })}
              </div>
              <div className="mt-1 text-sm text-base-content/70">
                {t("settings.appearanceSettings.compactColumns.description", {
                  defaultValue:
                    "Choose how many assets are shown per row when Compact layout is active.",
                })}
              </div>
            </div>
            <div className="badge badge-soft badge-neutral">
              {compactColumns}
            </div>
          </div>

          <div className="mt-4 w-full max-w-md">
            <input
              type="range"
              min={4}
              max={10}
              step={1}
              value={compactColumns}
              className="range"
              onChange={onChangeCompactColumns}
              disabled={currentLayout !== "compact"}
            />
            <div className="mt-2 flex justify-between px-2.5 text-xs text-base-content/50">
              {compactColumnMarks.map((mark) => (
                <span key={`tick-${mark}`}>|</span>
              ))}
            </div>
            <div className="mt-2 flex justify-between px-2.5 text-xs text-base-content/70">
              {compactColumnMarks.map((mark) => (
                <span key={mark}>{mark}</span>
              ))}
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
