export const settingsTabs = ["general", "server", "lumen", "runtime"] as const;

export type SettingsTab = (typeof settingsTabs)[number];

export function settingsTabAfterKey(current: SettingsTab, key: string): SettingsTab | null {
  const index = settingsTabs.indexOf(current);
  switch (key) {
    case "ArrowRight":
    case "ArrowDown":
      return settingsTabs[(index + 1) % settingsTabs.length];
    case "ArrowLeft":
    case "ArrowUp":
      return settingsTabs[(index - 1 + settingsTabs.length) % settingsTabs.length];
    case "Home":
      return settingsTabs[0];
    case "End":
      return settingsTabs.at(-1) ?? null;
    default:
      return null;
  }
}

export function settingsSaveLabelKey(
  tab: SettingsTab,
  confirmingMove: boolean,
): "validateApplyRestart" | "networkSave" | "confirmMove" | "saveChanges" {
  if (tab === "runtime") return "validateApplyRestart";
  if (tab === "server") return "networkSave";
  if (tab === "lumen" && confirmingMove) return "confirmMove";
  return "saveChanges";
}

export function settingsCloseAllowed(dirty: boolean, confirmDiscard: () => boolean): boolean {
  return !dirty || confirmDiscard();
}
