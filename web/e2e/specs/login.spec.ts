import { credentials, expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";

test("@smoke administrator signs in and reaches the library", async ({ page }) => {
  await new LoginPage(page).signIn(credentials.username, credentials.password);
  await expect(page.getByRole("navigation")).toBeVisible();
});
