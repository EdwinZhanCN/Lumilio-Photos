import type { InstalledPluginRecord } from "./types";

export const STUDIO_PLUGIN_INSTALL_STORAGE_KEY =
  "lumilio.studio.installed_plugins.v1";

function parseInstalledPlugins(raw: string | null): InstalledPluginRecord[] {
  if (!raw) return [];

  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];

    return parsed
      .filter((item): item is InstalledPluginRecord => {
        if (typeof item !== "object" || item === null) return false;
        const record = item as Record<string, unknown>;
        return (
          typeof record.pluginId === "string" &&
          typeof record.version === "string" &&
          typeof record.installedAt === "string"
        );
      })
      .sort((a, b) => a.pluginId.localeCompare(b.pluginId));
  } catch {
    return [];
  }
}

function writeInstalledPlugins(records: InstalledPluginRecord[]): void {
  localStorage.setItem(STUDIO_PLUGIN_INSTALL_STORAGE_KEY, JSON.stringify(records));
}

export function readInstalledPlugins(): InstalledPluginRecord[] {
  return parseInstalledPlugins(localStorage.getItem(STUDIO_PLUGIN_INSTALL_STORAGE_KEY));
}

export function installPluginRecord(
  pluginId: string,
  version: string,
): InstalledPluginRecord[] {
  const records = readInstalledPlugins();
  const next: InstalledPluginRecord = {
    pluginId,
    version,
    installedAt: new Date().toISOString(),
  };

  const merged = [
    ...records.filter((item) => item.pluginId !== pluginId),
    next,
  ].sort((a, b) => a.pluginId.localeCompare(b.pluginId));

  writeInstalledPlugins(merged);
  return merged;
}

export function uninstallPluginRecord(pluginId: string): InstalledPluginRecord[] {
  const records = readInstalledPlugins();
  const next = records.filter((item) => item.pluginId !== pluginId);
  writeInstalledPlugins(next);
  return next;
}

export function isPluginInstalled(pluginId: string, version?: string): boolean {
  const records = readInstalledPlugins();
  return records.some(
    (item) => item.pluginId === pluginId && (!version || item.version === version),
  );
}
