import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PluginsWorkspacePanel } from "./PluginsWorkspacePanel";
import type {
  InstalledPluginRecord,
  StudioPluginUiModule,
} from "@/features/studio/plugins/types";

vi.mock("@/lib/i18n.tsx", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

afterEach(() => {
  cleanup();
});

const installedPlugins: InstalledPluginRecord[] = [
  {
    pluginId: "com.lumilio.border",
    version: "0.2.0",
    installedAt: "2026-02-17T00:00:00.000Z",
  },
  {
    pluginId: "com.lumilio.hello",
    version: "0.1.0",
    installedAt: "2026-02-17T00:00:00.000Z",
  },
];

const createUiModule = (): StudioPluginUiModule => ({
  meta: {
    id: "com.lumilio.border",
    version: "0.2.0",
    displayName: "Lumilio Border",
    mount: {
      panel: "plugins",
      order: 10,
    },
  },
  defaultParams: {
    strength: 0.7,
  },
  Panel: ({ value }) => (
    <div data-testid="plugin-panel">strength:{String(value.strength)}</div>
  ),
});

describe("PluginsWorkspacePanel", () => {
  it("switches active plugin through tabs", () => {
    const onSelectPlugin = vi.fn();

    render(
      <PluginsWorkspacePanel
        isGenerating={false}
        onGeneratePlugin={vi.fn()}
        pluginRuntimeEnabled={true}
        installedPlugins={installedPlugins}
        selectedPluginId="com.lumilio.border"
        onSelectPlugin={onSelectPlugin}
        pluginUiModule={createUiModule()}
        pluginParams={{ strength: 0.7 }}
        onPluginParamsChange={vi.fn()}
        pluginLoading={false}
        pluginError={null}
        onOpenMarketplace={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "com.lumilio.hello@0.1.0" }));

    expect(onSelectPlugin).toHaveBeenCalledWith("com.lumilio.hello");
    expect(screen.getByTestId("plugin-panel")).toBeInTheDocument();
  });

  it("toggles apply button by readiness and runs plugin", () => {
    const onGeneratePlugin = vi.fn();
    const baseProps = {
      isGenerating: false,
      onGeneratePlugin,
      pluginRuntimeEnabled: true,
      installedPlugins,
      onSelectPlugin: vi.fn(),
      onPluginParamsChange: vi.fn(),
      pluginLoading: false,
      pluginError: null,
      onOpenMarketplace: vi.fn(),
    };

    const { rerender } = render(
      <PluginsWorkspacePanel
        {...baseProps}
        selectedPluginId={null}
        pluginUiModule={null}
        pluginParams={{}}
      />,
    );

    expect(
      screen.getByRole("button", { name: "studio.plugins.apply" }),
    ).toBeDisabled();

    rerender(
      <PluginsWorkspacePanel
        {...baseProps}
        selectedPluginId="com.lumilio.border"
        pluginUiModule={createUiModule()}
        pluginParams={{ strength: 0.7 }}
      />,
    );

    const applyButton = screen.getByRole("button", { name: "studio.plugins.apply" });
    expect(applyButton).toBeEnabled();

    fireEvent.click(applyButton);
    expect(onGeneratePlugin).toHaveBeenCalledTimes(1);
  });

  it("shows marketplace shortcut when no plugin is installed", () => {
    const onOpenMarketplace = vi.fn();

    render(
      <PluginsWorkspacePanel
        isGenerating={false}
        onGeneratePlugin={vi.fn()}
        pluginRuntimeEnabled={true}
        installedPlugins={[]}
        selectedPluginId={null}
        onSelectPlugin={vi.fn()}
        pluginUiModule={null}
        pluginParams={{}}
        onPluginParamsChange={vi.fn()}
        pluginLoading={false}
        pluginError={null}
        onOpenMarketplace={onOpenMarketplace}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "studio.plugins.openMarketplace" }),
    );
    expect(onOpenMarketplace).toHaveBeenCalledTimes(1);
  });
});
