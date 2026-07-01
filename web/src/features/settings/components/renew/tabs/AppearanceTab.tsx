import React from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { usePreference } from "@/features/settings";
import { useResolvedThemeMode, useThemePreference } from "@/lib/theme";
import {
  DAISYUI_DARK_THEMES,
  DAISYUI_LIGHT_THEMES,
  type DaisyUIDarkThemeName,
  type DaisyUILightThemeName,
} from "@/lib/theme/daisyuiThemes";
import {
  Columns3Icon,
  LanguagesIcon,
  LayoutGridIcon,
  MapPinIcon,
  MonitorIcon,
  MoonIcon,
  RowsIcon,
  SunIcon,
} from "lucide-react";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../SettingsGroup";
import { SettingsDropdown } from "../SettingsDropdown";
import { ThemePicker, type ModeThemeName } from "../ThemePicker";

export default function AppearanceTab() {
  const { t } = useI18n();

  const [language, setLanguage] = usePreference("language");
  const [region, setRegion] = usePreference("region");
  const [theme, setTheme] = useThemePreference();
  const [assetPage, setAssetPage] = usePreference("assetPage");
  const resolvedThemeMode = useResolvedThemeMode();

  const isFollowingSystem = theme.followSystem;
  const currentThemeMode = resolvedThemeMode;
  const currentLayout = assetPage.layout;
  const compactColumns = assetPage.columns;

  const onChangeFollowSystem = (e: React.ChangeEvent<HTMLInputElement>) => {
    const nextFollowSystem = e.target.checked;
    if (!nextFollowSystem) {
      setTheme({ ...theme, followSystem: false, mode: resolvedThemeMode });
      return;
    }
    setTheme({ ...theme, followSystem: nextFollowSystem });
  };

  const onChangeLightModeTheme = (nextTheme: ModeThemeName) =>
    setTheme({
      ...theme,
      themes: { ...theme.themes, light: nextTheme as DaisyUILightThemeName },
    });

  const onChangeDarkModeTheme = (nextTheme: ModeThemeName) =>
    setTheme({
      ...theme,
      themes: { ...theme.themes, dark: nextTheme as DaisyUIDarkThemeName },
    });

  const layoutOptions = [
    {
      value: "compact" as const,
      icon: <LayoutGridIcon className="size-4" />,
      color: "bg-warning text-warning-content",
      label: t("settings.appearanceSettings.layoutOptions.compact.label"),
      description: t("settings.appearanceSettings.layoutOptions.compact.description"),
    },
    {
      value: "full" as const,
      icon: <RowsIcon className="size-4" />,
      color: "bg-secondary text-secondary-content",
      label: t("settings.appearanceSettings.layoutOptions.full.label"),
      description: t("settings.appearanceSettings.layoutOptions.full.description"),
    },
  ];

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      <SettingsGroup
        title={t("settings.languageAndRegion")}
        description={t("settings.appearanceSettings.languageRegionDescription", {
          defaultValue: "Interface display language and regional formats.",
        })}
      >
        <SettingsRow
          htmlFor="ui-language"
          icon={<LanguagesIcon className="size-4" />}
          iconColor="bg-info text-info-content"
          label={t("settings.language")}
          control={
            <SettingsDropdown
              id="ui-language"
              value={language}
              options={[
                { value: "en", label: "English" },
                { value: "zh", label: "中文" },
              ]}
              onChange={(nextLanguage) => setLanguage(nextLanguage)}
              ariaLabel={t("settings.language")}
              className="w-32"
            />
          }
        />
        <SettingsRow
          htmlFor="ui-region"
          icon={<MapPinIcon className="size-4" />}
          iconColor="bg-success text-success-content"
          label={t("settings.region")}
          control={
            <SettingsDropdown
              id="ui-region"
              value={region}
              options={[
                { value: "china", label: t("settings.regionOptions.china") },
                { value: "other", label: t("settings.regionOptions.other") },
              ]}
              onChange={(nextRegion) => setRegion(nextRegion)}
              ariaLabel={t("settings.region")}
              className="w-32"
            />
          }
        />
      </SettingsGroup>

      <SettingsGroup
        title={t("settings.appearanceSettings.themes.title")}
        description={t("settings.appearanceSettings.themes.description")}
      >
        <SettingsRow
          htmlFor="ui-follow-system"
          icon={<MonitorIcon className="size-4" />}
          iconColor="bg-primary text-primary-content"
          label={t("settings.appearanceSettings.themes.followSystem.label")}
          description={t("settings.appearanceSettings.themes.followSystem.description")}
          control={
            <input
              id="ui-follow-system"
              type="checkbox"
              className="toggle toggle-primary"
              checked={isFollowingSystem}
              onChange={onChangeFollowSystem}
            />
          }
        />
        <SettingsBlock>
          <ThemePicker
            title={t("settings.appearanceSettings.themes.light.label")}
            description={t("settings.appearanceSettings.themes.light.description")}
            activeBadgeLabel={t("settings.appearanceSettings.themes.activeBadge")}
            isActiveMode={currentThemeMode === "light"}
            themes={DAISYUI_LIGHT_THEMES}
            selectedTheme={theme.themes.light}
            icon={SunIcon}
            onSelect={onChangeLightModeTheme}
          />
        </SettingsBlock>
        <SettingsBlock>
          <ThemePicker
            title={t("settings.appearanceSettings.themes.dark.label")}
            description={t("settings.appearanceSettings.themes.dark.description")}
            activeBadgeLabel={t("settings.appearanceSettings.themes.activeBadge")}
            isActiveMode={currentThemeMode === "dark"}
            themes={DAISYUI_DARK_THEMES}
            selectedTheme={theme.themes.dark}
            icon={MoonIcon}
            onSelect={onChangeDarkModeTheme}
          />
        </SettingsBlock>
      </SettingsGroup>

      <SettingsGroup
        title={t("settings.assetPage")}
        description={t("settings.appearanceSettings.layoutDescription")}
      >
        {layoutOptions.map((option) => (
          <SettingsRow
            key={option.value}
            icon={option.icon}
            iconColor={option.color}
            label={option.label}
            description={option.description}
            selected={currentLayout === option.value}
            onClick={() => setAssetPage({ ...assetPage, layout: option.value })}
          />
        ))}
        <SettingsBlock className={currentLayout === "compact" ? "" : "opacity-50"}>
          <div className="flex items-center gap-3">
            <span className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-accent text-accent-content">
              <Columns3Icon className="size-4" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium">
                {t("settings.appearanceSettings.compactColumns.label", {
                  defaultValue: "Compact columns",
                })}
              </div>
              <div className="text-xs text-base-content/55">
                {t("settings.appearanceSettings.compactColumns.description", {
                  defaultValue:
                    "Choose how many assets are shown per row when Compact layout is active.",
                })}
              </div>
            </div>
            <span className="badge badge-soft badge-neutral">{compactColumns}</span>
          </div>
          <div className="mt-2">
            <input
              type="range"
              min={4}
              max={10}
              step={1}
              value={compactColumns}
              className="range range-primary range-xs"
              onChange={(e) => setAssetPage({ ...assetPage, columns: Number(e.target.value) })}
              disabled={currentLayout !== "compact"}
            />
          </div>
        </SettingsBlock>
      </SettingsGroup>
    </div>
  );
}
