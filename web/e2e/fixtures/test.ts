import { createHash, randomUUID } from "node:crypto";
import { test as base, expect } from "playwright/test";
import { provisionWorkspace, type Workspace } from "../support/workspace";

// Separate invocations against the same disposable stack must not reuse state.
const invocationIdentity = randomUUID();

/**
 * Every attempt gets its own admin user and repository. Including the test,
 * repeat and retry in the identity prevents a failed attempt from leaving
 * mutable server state that changes the result of the next attempt.
 */
export const test = base.extend<{ workspace: Workspace }>({
  workspace: [
    async ({ browserName }, use, testInfo) => {
      const identity = [
        invocationIdentity,
        browserName,
        testInfo.testId,
        testInfo.repeatEachIndex,
        testInfo.retry,
      ].join(":");
      const attemptIndex = Number.parseInt(
        createHash("sha256").update(identity).digest("hex").slice(0, 10),
        16,
      );
      await use(await provisionWorkspace(attemptIndex));
    },
    { timeout: 120_000 },
  ],
});

export { expect };
export type { Workspace };
