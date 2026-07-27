import { describe, expect, it } from "vite-plus/test";
import {
  settingsCloseAllowed,
  settingsSaveLabelKey,
  settingsTabAfterKey,
} from "./settings-dialog.ts";

describe("settings dialog model", () => {
  it("supports cyclic arrow-key tab navigation", () => {
    expect(settingsTabAfterKey("general", "ArrowRight")).toBe("server");
    expect(settingsTabAfterKey("runtime", "ArrowRight")).toBe("general");
    expect(settingsTabAfterKey("general", "ArrowLeft")).toBe("runtime");
    expect(settingsTabAfterKey("lumen", "ArrowUp")).toBe("server");
  });

  it("supports Home and End and ignores unrelated keys", () => {
    expect(settingsTabAfterKey("lumen", "Home")).toBe("general");
    expect(settingsTabAfterKey("general", "End")).toBe("runtime");
    expect(settingsTabAfterKey("server", "Enter")).toBeNull();
  });

  it("selects the semantic footer action for every tab", () => {
    expect(settingsSaveLabelKey("general", false)).toBe("saveChanges");
    expect(settingsSaveLabelKey("server", false)).toBe("networkSave");
    expect(settingsSaveLabelKey("lumen", true)).toBe("confirmMove");
    expect(settingsSaveLabelKey("runtime", false)).toBe("validateApplyRestart");
  });

  it("closes clean sessions directly and confirms dirty sessions", () => {
    let confirmations = 0;
    expect(
      settingsCloseAllowed(false, () => {
        confirmations += 1;
        return false;
      }),
    ).toBe(true);
    expect(confirmations).toBe(0);
    expect(
      settingsCloseAllowed(true, () => {
        confirmations += 1;
        return false;
      }),
    ).toBe(false);
    expect(settingsCloseAllowed(true, () => true)).toBe(true);
  });
});
