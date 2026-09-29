import { useTranslation } from "react-i18next";
import {
  type DesktopPreferences,
  type RuntimeConfigSettings,
} from "../../bindings/desktop/internal/control/dto/models.js";
import { Button } from "@/components/motion/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/motion/select";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";

export function LanguageSelect({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select
      value={preferences.locale}
      onValueChange={(value) => update("locale", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("general.language")} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="en">English</SelectItem>
        <SelectItem value="zh-CN">简体中文</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function ThemeStatusControl({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const system = !preferences.theme || preferences.theme === "system";
  const currentLabel = system
    ? t("general.themeSystem")
    : preferences.theme === "dark"
      ? t("general.themeDark")
      : t("general.themeLight");
  return (
    <div className="flex items-center justify-end gap-2">
      <span className="setting-value">{currentLabel}</span>
      {!system ? (
        <Button
          variant="ghost"
          size="sm"
          disabled={disabled}
          onClick={() => update("theme", "system")}
        >
          {t("general.themeSystem")}
        </Button>
      ) : null}
    </div>
  );
}

export function RegionSelect({
  preferences,
  update,
  disabled,
}: {
  preferences: DesktopPreferences;
  update: SettingsDraftController["updatePreference"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select
      value={preferences.region}
      onValueChange={(value) => update("region", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("general.region")} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="global">{t("region.global")}</SelectItem>
        <SelectItem value="china">{t("region.china")}</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function NetworkSelect({
  settings,
  update,
  disabled,
}: {
  settings: RuntimeConfigSettings;
  update: SettingsDraftController["updateRuntime"];
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  return (
    <Select
      value={settings.networkMode}
      onValueChange={(value) => update("networkMode", value)}
      disabled={disabled}
      className="compact-select"
    >
      <SelectTrigger>
        <SelectValue placeholder={t("network.access")} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value="local">{t("network.local")}</SelectItem>
        <SelectItem value="lan">{t("network.lan")}</SelectItem>
      </SelectContent>
    </Select>
  );
}

export function networkLabel(mode: string, t: (key: string) => string) {
  if (mode === "lan") return t("network.lan");
  if (mode === "custom") return t("network.custom");
  return t("network.local");
}
