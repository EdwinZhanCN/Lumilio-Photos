import { MemoryRouter } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { renderWithProviders } from "@test/render";
import RoleRoute from "./RoleRoute";

const useAuthMock = vi.fn();

vi.mock("../../state/useAuth", () => ({
  useAuth: () => useAuthMock(),
}));

describe("RoleRoute", () => {
  beforeEach(() => useAuthMock.mockReset());

  it("renders an admin-only route for administrators", async () => {
    useAuthMock.mockReturnValue({ user: { role: "admin" } });
    const screen = renderWithProviders(
      <MemoryRouter>
        <RoleRoute requiredRole="admin"><p>monitor</p></RoleRoute>
      </MemoryRouter>,
      { router: false },
    );
    const view = await screen;
    await expect.element(view.getByText("monitor")).toBeVisible();
  });

  it("does not expose an admin-only route to regular users", async () => {
    useAuthMock.mockReturnValue({ user: { role: "user" } });
    const screen = renderWithProviders(
      <MemoryRouter initialEntries={["/server-monitor"]}>
        <RoleRoute requiredRole="admin" fallback={<p>Page unavailable</p>}>
          <p>monitor</p>
        </RoleRoute>
      </MemoryRouter>,
      { router: false },
    );
    const view = await screen;
    await expect.element(view.getByText("monitor")).not.toBeInTheDocument();
    await expect.element(view.getByText("Page unavailable")).toBeVisible();
  });
});
