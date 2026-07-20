import { test as base, expect } from "playwright/test";
import { provisionWorkspace, type Workspace } from "../support/workspace";

/**
 * `workspace` is worker-scoped: each Playwright worker gets its own admin user
 * and repository, so parallel workers never scan or upload into shared state.
 */
// eslint-disable-next-line @typescript-eslint/no-empty-object-type -- Playwright's
// own signature for "no extra test-scoped fixtures".
export const test = base.extend<{}, { workspace: Workspace }>({
  workspace: [
    // eslint-disable-next-line no-empty-pattern -- Playwright parses this
    // parameter to infer fixture dependencies and rejects a named one.
    async ({}, use, workerInfo) => {
      await use(await provisionWorkspace(workerInfo.parallelIndex));
    },
    { scope: "worker" },
  ],
});

export { expect };
export type { Workspace };
