import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { StudioSidebar } from "./StudioSidebar";
import type { InstalledPluginRecord } from "@/features/studio/plugins/types";

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

describe("StudioSidebar", () => {
  it("renders installed plugins under the plugins parent and switches plugin selection", () => {
    const setActivePanel = vi.fn();
    const onSelectPlugin = vi.fn();

    render(
      <StudioSidebar
        activePanel="plugins"
        setActivePanel={setActivePanel}
        pluginRuntimeEnabled={true}
        installedPlugins={installedPlugins}
        selectedPluginId="com.lumilio.border"
        onSelectPlugin={onSelectPlugin}
      />,
    );

    expect(screen.getByText("studio.nav.plugins")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "com.lumilio.border" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "com.lumilio.hello" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "com.lumilio.hello" }));

    expect(setActivePanel).toHaveBeenCalledWith("plugins");
    expect(onSelectPlugin).toHaveBeenCalledWith("com.lumilio.hello");
  });

  it("falls back to a single plugins item when no installed plugin is available", () => {
    render(
      <StudioSidebar
        activePanel="plugins"
        setActivePanel={vi.fn()}
        pluginRuntimeEnabled={true}
        installedPlugins={[]}
        selectedPluginId={null}
        onSelectPlugin={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "studio.nav.plugins" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "com.lumilio.border" })).not.toBeInTheDocument();
  });
});
