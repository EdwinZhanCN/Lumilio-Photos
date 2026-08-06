import { spawnSync } from "node:child_process";
import { randomUUID } from "node:crypto";
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

type RestoreOperation = {
  id: string;
  status:
    | "staged"
    | "restart_requested"
    | "installing"
    | "verifying"
    | "completed"
    | "rolling_back"
    | "rolled_back"
    | "failed";
  message: string;
};

const repositoryRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../../..");
const compose = [
  "compose",
  "-f",
  path.join(repositoryRoot, "web/e2e/compose.yml"),
  "-p",
  "lumilio-photos-e2e",
];

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
  const repositories = (await api<RepositoryList>("/api/v1/repositories", { token })).repositories;
  return repositories?.some((repository) => repository.id === repositoryID) ?? false;
}

async function repositoryPresence(token: string, repositoryID: string): Promise<boolean | null> {
  try {
    return await repositoryExists(token, repositoryID);
  } catch {
    // A successful restore deliberately drains and recreates the HTTP runtime.
    // A transport reset proves neither presence nor absence; keep polling until
    // the new generation answers from the restored catalog.
    return null;
  }
}

/**
 * Failure injection is limited to the private backup directory. The restore
 * itself, and every state assertion, still cross the public admin API/UI; the
 * harness never edits the SQLite catalog directly.
 */
function writePrivateBackupFixture(target: string, input: string | Buffer): void {
  const result = spawnSync(
    "docker",
    [
      ...compose,
      "exec",
      "-T",
      "--user",
      "app",
      "lumilio",
      "sh",
      "-c",
      'umask 077; cat > "$1"',
      "backup-fixture",
      target,
    ],
    {
      cwd: repositoryRoot,
      input,
      stdio: ["pipe", "inherit", "inherit"],
    },
  );
  if (result.error) throw result.error;
  if (result.status !== 0) {
    throw new Error(`install corrupt backup fixture failed (${result.status})`);
  }
}

function installCorruptBackupFixture(): string {
  const name = "20991231T235959.000000Z-library.sqlite3";
  const corruptDatabase = Buffer.from("THIS IS NOT A SQLITE DATABASE\n");
  const manifestName = "20991231T235959.000000Z-library-manifest.json";
  const base = "/data/app-state/backups";
  writePrivateBackupFixture(`${base}/${name}`, corruptDatabase);
  writePrivateBackupFixture(
    `${base}/${manifestName}`,
    `${JSON.stringify(
      {
        format_version: 2,
        app_version: "e2e-corrupt",
        config_schema_version: 3,
        application_migration_version: 3,
        river_migration_version: 1,
        sqlite_version: "invalid-fixture",
        vec1_version: "invalid-fixture",
        created_at: "2099-12-31T23:59:59Z",
        database_size: corruptDatabase.length,
        sha256: "0".repeat(64),
        quick_check: "ok",
        foreign_key_violations: 0,
        library_id: "invalid-fixture",
      },
      null,
      2,
    )}\n`,
  );
  return name;
}

async function restoreFromRow(page: Page, row: Locator): Promise<string> {
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
  expect(response.status()).toBe(202);
  const operation = (await response.json()) as RestoreOperation;
  expect(operation.id).toBeTruthy();
  return operation.id;
}

async function rejectRestoreFromRow(page: Page, row: Locator): Promise<void> {
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
  expect(response.status()).toBe(500);
}

async function readRestoreOperation(token: string, operationID: string): Promise<RestoreOperation | null> {
  try {
    return await api<RestoreOperation>(
      `/api/v1/settings/backup-restores/${encodeURIComponent(operationID)}`,
      { token },
    );
  } catch {
    // The active HTTP generation disappears while the catalog is swapped. A
    // transport failure is an expected intermediate state, not a terminal one.
    return null;
  }
}

async function waitForRestoreTerminal(
  token: string,
  operationID: string,
): Promise<RestoreOperation> {
  let terminal: RestoreOperation | null = null;
  await expect
    .poll(
      async () => {
        const operation = await readRestoreOperation(token, operationID);
        if (
          operation &&
          (operation.status === "completed" ||
            operation.status === "rolled_back" ||
            operation.status === "failed")
        ) {
          terminal = operation;
        }
        return terminal?.status;
      },
      {
        message: `restore operation ${operationID} should reach a terminal receipt`,
        timeout: 90_000,
        intervals: [500, 1_000, 2_000, 3_000],
      },
    )
    .toBeTruthy();
  if (!terminal) throw new Error(`restore operation ${operationID} never became terminal`);
  return terminal;
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
  if (!routineName) throw new Error("on-demand backup did not produce a routine snapshot");

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

  const afterBackup = await createRepository(
    workspace.token,
    `Recovery success mutation ${randomUUID()}`,
  );
  expect(await repositoryExists(workspace.token, afterBackup.id)).toBe(true);

  const successfulOperationID = await restoreFromRow(page, routineRow);
  const successfulOperation = await waitForRestoreTerminal(workspace.token, successfulOperationID);
  expect(successfulOperation.status).toBe("completed");
  await expect
    .poll(() => repositoryPresence(workspace.token, afterBackup.id), {
      message: "successful restore should remove data created after the dump",
      timeout: 30_000,
    })
    .toBe(false);

  const afterSuccess = await listBackups(workspace.token);
  expect(afterSuccess.some((entry) => entry.restore_point)).toBe(true);

  const corruptName = installCorruptBackupFixture();
  const rollbackProof = await createRepository(
    workspace.token,
    `Recovery rollback proof ${randomUUID()}`,
  );
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
  await rejectRestoreFromRow(page, corruptRow);
  await expect(page.getByText("Restore could not be staged", { exact: true })).toBeVisible();
  await expect
    .poll(() => repositoryPresence(workspace.token, rollbackProof.id), {
      message: "failed restore should preserve the pre-restore public state",
      timeout: 30_000,
    })
    .toBe(true);
  expect((await listBackups(workspace.token)).some((entry) => entry.restore_point)).toBe(true);
});
