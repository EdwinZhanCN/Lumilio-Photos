import { describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import { seedSession } from "@test/session";
import { t } from "@test/i18n";
import "@/styles/App.css";
import SettingsShell from "./SettingsShell";

vi.mock("../account/AccountTab", () => ({ default: () => <div>account-tab</div> }));
vi.mock("../ai/AiTab", () => ({ default: () => <div>ai-tab</div> }));
vi.mock("../appearance/AppearanceTab", () => ({ default: () => <div>appearance-tab</div> }));
vi.mock("../about/AboutTab", () => ({ default: () => <div>about-tab</div> }));
vi.mock("../cloud/CloudTab", () => ({ default: () => <div>cloud-tab</div> }));
vi.mock("../server/ServerTab", () => ({ default: () => <div>server-tab</div> }));
vi.mock("../users/UsersTab", () => ({ default: () => <div>users-tab</div> }));

describe("SettingsShell", () => {
  it("keeps the tab strip on one horizontally scrollable row", async () => {
    seedSession({ user_id: 1, username: "member", role: "user" });
    const screen = await renderWithProviders(
      <div style={{ width: 320 }}>
        <SettingsShell />
      </div>,
      { auth: true },
    );

    const tabList = screen.getByRole("tablist", { name: t("settings.title") });
    await expect.element(tabList).toBeVisible();

    const tabListElement = tabList.element() as HTMLElement;
    const styles = getComputedStyle(tabListElement);
    expect(styles.flexWrap).toBe("nowrap");
    expect(styles.overflowX).toBe("auto");
    expect(tabListElement.scrollWidth).toBeGreaterThan(tabListElement.clientWidth);
  });
});
