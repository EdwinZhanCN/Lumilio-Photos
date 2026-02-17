import { useCallback, useEffect, useMemo, useState } from "react";
import {
  STUDIO_PLUGIN_INSTALL_STORAGE_KEY,
  installPluginRecord,
  isPluginInstalled,
  readInstalledPlugins,
  uninstallPluginRecord,
} from "@/features/studio/plugins/installStore";
import type { InstalledPluginRecord } from "@/features/studio/plugins/types";

export interface UseStudioPluginInstallResult {
  installed: InstalledPluginRecord[];
  install: (pluginId: string, version: string) => void;
  uninstall: (pluginId: string) => void;
  isInstalled: (pluginId: string, version?: string) => boolean;
  installedById: Record<string, InstalledPluginRecord>;
}

export function useStudioPluginInstall(): UseStudioPluginInstallResult {
  const [installed, setInstalled] = useState<InstalledPluginRecord[]>(() =>
    readInstalledPlugins(),
  );

  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.key === STUDIO_PLUGIN_INSTALL_STORAGE_KEY) {
        setInstalled(readInstalledPlugins());
      }
    };

    window.addEventListener("storage", onStorage);
    return () => {
      window.removeEventListener("storage", onStorage);
    };
  }, []);

  const install = useCallback((pluginId: string, version: string) => {
    const next = installPluginRecord(pluginId, version);
    setInstalled(next);
  }, []);

  const uninstall = useCallback((pluginId: string) => {
    const next = uninstallPluginRecord(pluginId);
    setInstalled(next);
  }, []);

  const isInstalledFn = useCallback((pluginId: string, version?: string) => {
    return isPluginInstalled(pluginId, version);
  }, []);

  const installedById = useMemo(() => {
    return installed.reduce<Record<string, InstalledPluginRecord>>((acc, item) => {
      acc[item.pluginId] = item;
      return acc;
    }, {});
  }, [installed]);

  return {
    installed,
    install,
    uninstall,
    isInstalled: isInstalledFn,
    installedById,
  };
}
