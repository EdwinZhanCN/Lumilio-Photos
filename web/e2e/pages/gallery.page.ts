import type { Page } from "playwright/test";
import { t } from "../support/i18n";

export class GalleryPage {
  constructor(private readonly page: Page) {}

  /**
   * Narrows the gallery to one repository. The default view spans every
   * repository, so assertions that skip this depend on how much unrelated media
   * the instance happens to hold.
   */
  async scopeTo(repositoryName: string) {
    await this.page.goto("/assets");
    // The scope select is rendered twice for the responsive layouts; only one is
    // visible at a given viewport.
    await this.page
      .getByLabel(t("assets.assetsPageHeader.scope.title"))
      .filter({ visible: true })
      .selectOption({ label: repositoryName });
  }
}
