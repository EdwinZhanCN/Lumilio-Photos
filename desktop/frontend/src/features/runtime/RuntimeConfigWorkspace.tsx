import { FolderOpen } from "lucide-react";
import { useTranslation } from "react-i18next";
import { DesktopService } from "../../../bindings/desktop/internal/control/index.js";
import type { RuntimeConfigSettings } from "../../../bindings/desktop/internal/control/dto/models.js";
import { Button } from "@/components/motion/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/motion/select";
import { SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import type { ToastInput } from "@/components/motion/animated-toast-stack";
import { errorMessage } from "@/lib/desktop/errors";

export function RuntimeConfigWorkspace({
  settings,
  disabled,
  updateSetting,
  showToast,
}: {
  settings: RuntimeConfigSettings;
  disabled?: boolean;
  updateSetting: <K extends keyof RuntimeConfigSettings>(key: K, value: RuntimeConfigSettings[K]) => void;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const openManifest = async () => {
    try {
      await DesktopService.OpenRuntimeManifest();
    } catch (reason: unknown) {
      showToast({ title: t("runtimeWorkspace.manifestFailed"), description: errorMessage(reason), status: "error" });
    }
  };

  return (
    <>
      <SettingsSection title={t("runtimeWorkspace.configuration")}>
        <SettingRow title={t("runtimeWorkspace.networkAccess")} description={t("runtimeWorkspace.networkDescription")}>
          <Select
            value={settings.networkMode}
            onValueChange={(value) => updateSetting("networkMode", value)}
            disabled={disabled}
            className="compact-select"
          >
            <SelectTrigger><SelectValue placeholder={t("runtimeWorkspace.networkAccess")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="local">{t("network.local")}</SelectItem>
              <SelectItem value="lan">{t("network.lan")}</SelectItem>
              {settings.networkMode === "custom" ? <SelectItem value="custom">{t("network.custom")}</SelectItem> : null}
            </SelectContent>
          </Select>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title={t("runtimeWorkspace.processing")}>
        <SettingRow title={t("runtimeWorkspace.hardwareAcceleration")} description={t("runtimeWorkspace.hardwareAccelerationDescription")}>
          <Select
            value={settings.hardwareAcceleration}
            onValueChange={(value) => updateSetting("hardwareAcceleration", value)}
            disabled={disabled}
            className="compact-select"
          >
            <SelectTrigger><SelectValue placeholder={t("runtimeWorkspace.acceleration")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">{t("runtimeWorkspace.auto")}</SelectItem>
              <SelectItem value="videotoolbox">{t("runtimeWorkspace.videoToolbox", "VideoToolbox")}</SelectItem>
              <SelectItem value="vaapi">{t("runtimeWorkspace.vaapi", "VA-API")}</SelectItem>
              <SelectItem value="nvenc">{t("runtimeWorkspace.nvenc", "NVIDIA NVENC")}</SelectItem>
              <SelectItem value="qsv">{t("runtimeWorkspace.qsv", "Intel Quick Sync")}</SelectItem>
              <SelectItem value="none">{t("runtimeWorkspace.softwareOnly")}</SelectItem>
            </SelectContent>
          </Select>
        </SettingRow>
        <SettingRow title={t("runtimeWorkspace.loggingLevel")} description={t("runtimeWorkspace.loggingLevelDescription")}>
          <Select
            value={settings.loggingLevel}
            onValueChange={(value) => updateSetting("loggingLevel", value)}
            disabled={disabled}
            className="compact-select"
          >
            <SelectTrigger><SelectValue placeholder={t("runtimeWorkspace.loggingLevel")} /></SelectTrigger>
            <SelectContent>
              <SelectItem value="debug">{t("runtimeWorkspace.debug")}</SelectItem>
              <SelectItem value="info">{t("runtimeWorkspace.info")}</SelectItem>
              <SelectItem value="warn">{t("runtimeWorkspace.warnings")}</SelectItem>
              <SelectItem value="error">{t("runtimeWorkspace.errors")}</SelectItem>
            </SelectContent>
          </Select>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title={t("runtimeWorkspace.advanced")}>
        <SettingRow title={t("runtimeWorkspace.manifest")} description={t("runtimeWorkspace.manifestDescription")}>
          <Button variant="secondary" size="sm" onClick={() => void openManifest()}>
            <FolderOpen className="size-3.5" /> {t("common.open")}
          </Button>
        </SettingRow>
      </SettingsSection>
    </>
  );
}
