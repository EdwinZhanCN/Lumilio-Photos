import { useTranslation } from "react-i18next";
import { Switch } from "@/components/motion/switch";
import { PageHeading, SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";
import { SettingsLoading } from "../shared";
import { LanguageSelect, RegionSelect, ThemeStatusControl } from "../settingsControls";

export function GeneralPanel({ draft }: { draft: SettingsDraftController }) {
  const { t } = useTranslation();
  if (!draft.preferences) return <SettingsLoading label={t("common.loadingPreferences")} />;
  const busy = draft.phase === "preparing" || draft.phase === "saving";
  return (
    <>
      <PageHeading title={t("general.title")} description={t("general.description")} />
      <SettingsSection title={t("general.display")}>
        <SettingRow title={t("general.language")} description={t("general.languageDescription")}>
          <LanguageSelect
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow title={t("general.region")} description={t("general.regionDescription")}>
          <RegionSelect
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow title={t("general.theme")} description={t("general.themeDescription")}>
          <ThemeStatusControl
            preferences={draft.preferences}
            update={draft.updatePreference}
            disabled={busy}
          />
        </SettingRow>
        <SettingRow
          title={t("general.openOnLaunch")}
          description={t("general.openOnLaunchDescription")}
        >
          <Switch
            checked={draft.preferences.openProductOnLaunch}
            onCheckedChange={(value) => draft.updatePreference("openProductOnLaunch", value)}
            disabled={busy}
            ariaLabel={t("general.openOnLaunch")}
          />
        </SettingRow>
      </SettingsSection>
    </>
  );
}
