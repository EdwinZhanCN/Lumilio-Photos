import type { Page } from "playwright/test";
import { expect } from "playwright/test";
import { t } from "../support/i18n";

export class LoginPage {
  constructor(private readonly page: Page) {}

  async signIn(username: string, password: string) {
    await this.page.goto("/login");
    await this.page.getByLabel(t("auth.login.username"), { exact: true }).fill(username);
    await this.page.getByRole("button", { name: t("auth.login.continue") }).click();
    await this.page.getByLabel(t("auth.login.password"), { exact: true }).fill(password);
    await this.page.getByRole("button", { name: t("auth.login.signIn") }).click();
    await expect(this.page).toHaveURL(/\/$/);
  }
}
