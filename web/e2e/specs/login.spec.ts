import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";

test("@smoke administrator signs in and reaches the library", async ({ page, workspace }) => {
  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await expect(page.getByRole("navigation")).toBeVisible();
});
