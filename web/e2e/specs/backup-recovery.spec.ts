import { spawnSync } from "node:child_process";
import { gzipSync } from "node:zlib";
import path from "node:path";
import { fileURLToPath } from "node:url";
import type { Locator, Page } from "playwright/test";
import { expect, test } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api } from "../support/api";
import { t } from "../support/i18n";

type BackupEntry = {
  name: string;
  restore_point: boolean;
};

type BackupList = {
  backups?: BackupEntry[];
};

type Repository = {
  id: string;
  name: string;
};

type RepositoryList = {
  repositories?: Repository[];
};

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const compose = ["compose", "-f", "docker-compose.e2e.yml", "-p", "lumilio-photos-e2e"];

function backupRow(page: Page, name: string): Locator {
  return page.getByRole("listitem").filter({ hasText: name });
}

async function listBackups(token: string): Promise<BackupEntry[]> {
  return (await api<BackupList>("/api/v1/settings/backups", { token })).backups ?? [];
}

async function createRepository(token: string, name: string): Promise<Repository> {
  const { repository } = await api<{ repository: Repository }>("/api/v1/repositories", {
    method: "POST",
    token,
    body: JSON.stringify({
      name,
      role: "regular",
      storage_strategy: "flat",
      duplicate_handling: "rename",
    }),
  });
  return repository;
}

async function repositoryExists(token: string, repositoryID: string): Promise<boolean> {
  const repositories = (await api<RepositoryList>("/api/v1/repositories", { token }))
    .repositories;
  return repositories?.some((repository) => repository.id === repositoryID) ?? false;
}

/**
 * Failure injection is limited to the private backup directory. The restore
 * itself, and every state assertion, still cross the public admin API/UI; the
 * harness never edits PostgreSQL directly.
 */
function installCorruptBackupFixture(): string {
  const name = "lumilio-db-backup-20991231T235959-ve2e-corrupt-pg18.0.sql.gz";
  const target = `/data/app-state/backups/${name}`;
  const result = spawnSync(
    "docker",
    [
      ...compose,
      "exec",
      "-T",
      "server",
      "sh",
      "-c",
      'umask 077; cat > "$1"',
      "backup-fixture",
      target,
    ],
    {
      cwd: repositoryRoot,
      input: gzipSync("THIS IS NOT SQL;\n"),
      stdio: ["pipe", "inherit", "inherit"],
    },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`install corrupt backup fixture failed (${result.status})`);
  }
  return name;
}

async function restoreFromRow(page: Page, row: Locator, expectedStatus: number) {
  const responsePromise = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname.endsWith("/restore"),
    { timeout: 120_000 },
  );
  await row
    .getByRole("button", {
      name: t("settings.serverSettings.backup.restore"),
      exact: true,
    })
    .click();
  await row
    .getByRole("button", {
      name: t("settings.serverSettings.backup.confirmYes"),
      exact: true,
    })
    .click();
  const response = await responsePromise;
  expect(response.status()).toBe(expectedStatus);
}

test("@backup-recovery admin UI proves backup, download, restore, and rollback", async ({
  page,
  workspace,
}) => {
  test.setTimeout(240_000);

  await new LoginPage(page).signIn(workspace.username, workspace.password);
  await page.goto("/settings?tab=server");
  await expect(
    page.getByText(t("settings.serverSettings.backup.title"), { exact: true }),
  ).toBeVisible();

  const backupsBeforeRequest = new Set(
    (await listBackups(workspace.token)).map((entry) => entry.name),
  );
  await page
    .getByRole("button", {
      name: t("settings.serverSettings.backup.createNow"),
      exact: true,
    })
    .click();

  let routineBackup: BackupEntry | undefined;
  await expect
    .poll(
      async () => {
        routineBackup = (await listBackups(workspace.token)).find(
          (entry) => !entry.restore_point && !backupsBeforeRequest.has(entry.name),
        );
        return routineBackup?.name;
      },
      {
        message: "the queued on-demand backup should finish",
        timeout: 60_000,
        intervals: [500, 1_000, 2_000],
      },
    )
    .toBeTruthy();
  const routineName = routineBackup?.name;
  if (!routineName) throw new Error("on-demand backup did not produce a routine dump");

  await page
    .getByRole("button", {
      name: t("settings.serverSettings.backup.refresh"),
      exact: true,
    })
    .click();
  const routineRow = backupRow(page, routineName);
  await expect(routineRow).toBeVisible();

  const downloadPromise = page.waitForEvent("download");
  await routineRow
    .getByRole("button", {
      name: t("settings.serverSettings.backup.download"),
      exact: true,
    })
    .click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toBe(routineName);
  expect(await download.failure()).toBeNull();

  const afterBackup = await createRepository(workspace.token, "Recovery success mutation");
  expect(await repositoryExists(workspace.token, afterBackup.id)).toBe(true);

  await restoreFromRow(page, routineRow, 200);
  await expect
    .poll(() => repositoryExists(workspace.token, afterBackup.id), {
      message: "successful restore should remove data created after the dump",
      timeout: 30_000,
    })
    .toBe(false);

  const afterSuccess = await listBackups(workspace.token);
  expect(afterSuccess.some((entry) => entry.restore_point)).toBe(true);

  const corruptName = installCorruptBackupFixture();
  const rollbackProof = await createRepository(workspace.token, "Recovery rollback proof");
  expect(await repositoryExists(workspace.token, rollbackProof.id)).toBe(true);

  await page.goto("/settings?tab=server");
  await page
    .getByRole("button", {
      name: t("settings.serverSettings.backup.refresh"),
      exact: true,
    })
    .click();
  const corruptRow = backupRow(page, corruptName);
  await expect(corruptRow).toBeVisible();
  await restoreFromRow(page, corruptRow, 500);

  await expect(
    page.getByText(t("settings.serverSettings.backup.restoreFailed"), {
      exact: true,
    }),
  ).toBeVisible();
  await expect
    .poll(() => repositoryExists(workspace.token, rollbackProof.id), {
      message: "failed restore should preserve the pre-restore public state",
      timeout: 30_000,
    })
    .toBe(true);
  expect((await listBackups(workspace.token)).some((entry) => entry.restore_point)).toBe(true);
});
