import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MarketplacePanel } from "./MarketplacePanel";
import type { CatalogPluginSummary, InstalledPluginRecord } from "@/features/studio/plugins/types";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

afterEach(() => {
  cleanup();
});

const catalogPlugins: CatalogPluginSummary[] = [
  {
    id: "com.lumilio.border",
    displayName: "Lumilio Border",
    description: "Border effects",
    panel: "plugins",
    latestVersion: "0.2.0",
  },
  {
    id: "com.lumilio.hello",
    displayName: "Hello Lumilio",
    description: "Demo plugin",
    panel: "plugins",
    latestVersion: "0.1.0",
  },
];

const installedPlugins: InstalledPluginRecord[] = [
  {
    pluginId: "com.lumilio.border",
    version: "0.2.0",
    installedAt: "2026-02-17T00:00:00.000Z",
  },
];

describe("MarketplacePanel", () => {
  it("filters by installed status when installed-only is enabled", () => {
    render(
      <MarketplacePanel
        isGenerating={false}
        pluginRuntimeEnabled={true}
        installedPlugins={installedPlugins}
        catalogPlugins={catalogPlugins}
        onInstallPlugin={vi.fn()}
        onUninstallPlugin={vi.fn()}
        isPluginInstalled={(pluginId, version) =>
          installedPlugins.some(
            (item) => item.pluginId === pluginId && item.version === version,
          )
        }
        catalogLoading={false}
        catalogError={null}
      />,
    );

    expect(screen.getByText("Lumilio Border")).toBeInTheDocument();
    expect(screen.getByText("Hello Lumilio")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("studio.marketplace.installedOnly"));

    expect(screen.getByText("Lumilio Border")).toBeInTheDocument();
    expect(screen.queryByText("Hello Lumilio")).not.toBeInTheDocument();
  });

  it("invokes install and uninstall actions", () => {
    const onInstallPlugin = vi.fn();
    const onUninstallPlugin = vi.fn();

    render(
      <MarketplacePanel
        isGenerating={false}
        pluginRuntimeEnabled={true}
        installedPlugins={installedPlugins}
        catalogPlugins={catalogPlugins}
        onInstallPlugin={onInstallPlugin}
        onUninstallPlugin={onUninstallPlugin}
        isPluginInstalled={(pluginId, version) =>
          installedPlugins.some(
            (item) => item.pluginId === pluginId && item.version === version,
          )
        }
        catalogLoading={false}
        catalogError={null}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "studio.marketplace.uninstall" }));
    fireEvent.click(screen.getByRole("button", { name: "studio.marketplace.install" }));

    expect(onUninstallPlugin).toHaveBeenCalledWith("com.lumilio.border");
    expect(onInstallPlugin).toHaveBeenCalledWith("com.lumilio.hello", "0.1.0");
  });
});
