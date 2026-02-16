import { beforeEach, describe, expect, it } from "vitest";
import {
  STUDIO_PLUGIN_INSTALL_STORAGE_KEY,
  installPluginRecord,
  isPluginInstalled,
  readInstalledPlugins,
  uninstallPluginRecord,
} from "./installStore";

describe("installStore", () => {
  beforeEach(() => {
    localStorage.removeItem(STUDIO_PLUGIN_INSTALL_STORAGE_KEY);
  });

  it("installs and reads plugin records", () => {
    installPluginRecord("com.lumilio.border", "0.1.0");

    const installed = readInstalledPlugins();
    expect(installed).toHaveLength(1);
    expect(installed[0].pluginId).toBe("com.lumilio.border");
    expect(installed[0].version).toBe("0.1.0");
  });

  it("replaces old version on re-install", () => {
    installPluginRecord("com.lumilio.border", "0.1.0");
    installPluginRecord("com.lumilio.border", "0.2.0");

    const installed = readInstalledPlugins();
    expect(installed).toHaveLength(1);
    expect(installed[0].version).toBe("0.2.0");
  });

  it("uninstalls plugin", () => {
    installPluginRecord("com.lumilio.border", "0.1.0");
    uninstallPluginRecord("com.lumilio.border");

    expect(readInstalledPlugins()).toHaveLength(0);
    expect(isPluginInstalled("com.lumilio.border")).toBe(false);
  });

  it("self-heals from malformed storage JSON", () => {
    localStorage.setItem(STUDIO_PLUGIN_INSTALL_STORAGE_KEY, "bad-json");

    expect(readInstalledPlugins()).toEqual([]);
  });
});
