import type { APIRequestContext } from "playwright/test";
import { expect, test, type Workspace } from "../fixtures/test";
import { LoginPage } from "../pages/login.page";
import { api, baseURL } from "../support/api";
import {
  PEOPLE_PERSON_A_ASSETS,
  PEOPLE_PERSON_B_ASSETS,
  PEOPLE_PROFILE,
  selectedProfileAsset,
  uniqueJpeg,
} from "../support/assets";
import { t } from "../support/i18n";
import type { components } from "../../src/lib/http-commons/schema.d.ts";

type Upload = components["schemas"]["dto.UploadResponseDTO"];
type Operations = components["schemas"]["dto.UploadOperationStatusResponseDTO"];
type SystemSettings = components["schemas"]["dto.SystemSettingsDTO"];
type PeopleList = components["schemas"]["dto.ListPeopleResponseDTO"];
type PersonFaces = components["schemas"]["dto.ListPersonFacesResponseDTO"];
type PersonDetail = components["schemas"]["dto.PersonDetailDTO"];
type BrowseResponse = components["schemas"]["dto.QueryAssetsResponseDTO"];

// Face recognition runs on River workers after ingest and clusters
// incrementally; a low-power E2E host needs a wide bound for six portraits.
const PEOPLE_TIMEOUT = 180_000;

/** Uploads one pixel-identical, content-unique copy of a pinned portrait. */
async function uploadPortrait(
  request: APIRequestContext,
  workspace: Workspace,
  assetId: string,
  filename: string,
) {
  const response = await request.post(`${baseURL}/api/v1/assets`, {
    headers: { authorization: `Bearer ${workspace.token}` },
    multipart: {
      repository_id: workspace.repositoryId,
      file: {
        name: filename,
        mimeType: "image/jpeg",
        buffer: uniqueJpeg(
          selectedProfileAsset(PEOPLE_PROFILE, assetId),
          `lumilio-people:${filename}`,
        ),
      },
    },
  });
  expect(response.ok()).toBe(true);
  const upload = (await response.json()) as Upload;
  if (!upload.receipt_id) throw new Error(`upload of ${filename} returned no receipt`);
  return upload.receipt_id;
}

/** Waits until every receipt reached a successful terminal ingest state. */
async function waitForIngest(token: string, receipts: string[]) {
  await expect(async () => {
    const { operations } = await api<Operations>(
      `/api/v1/assets/batch/operations?receipt_ids=${receipts.join(",")}`,
      { token },
    );
    expect(operations?.map((operation) => operation.receipt_id).sort()).toEqual(
      [...receipts].sort(),
    );
    expect(operations?.every((operation) => operation.terminal && operation.success)).toBe(true);
  }).toPass({ timeout: PEOPLE_TIMEOUT });
}

async function assetIdByFilename(workspace: Workspace, filename: string) {
  const response = await api<BrowseResponse>("/api/v1/assets/list", {
    method: "POST",
    token: workspace.token,
    body: JSON.stringify({
      query: filename,
      search_type: "filename",
      filter: { repository_id: workspace.repositoryId },
      pagination: { limit: 20, offset: 0 },
      stack_mode: "expanded",
    }),
  });
  const assetId = response.items
    ?.map((item) => item.media_item?.primary_asset)
    .find((asset) => asset?.original_filename === filename)?.asset_id;
  if (!assetId) throw new Error(`ingested ${filename} is missing from the browse API`);
  return assetId;
}

async function peopleInRepository(workspace: Workspace) {
  const { people } = await api<PeopleList>(
    `/api/v1/people?repository_id=${workspace.repositoryId}&limit=100`,
    { token: workspace.token },
  );
  return people ?? [];
}

/** The asset IDs whose faces belong to a person, scoped to this repository. */
async function personAssetIds(workspace: Workspace, personId: number) {
  const { faces } = await api<PersonFaces>(
    `/api/v1/people/${personId}/faces?repository_id=${workspace.repositoryId}&limit=200`,
    { token: workspace.token },
  );
  return (faces ?? []).map((face) => face.asset_id!).sort();
}

test("@people two recognised people merge into one through the edit dialog", async ({
  page,
  request,
  workspace,
}) => {
  test.setTimeout(420_000);

  const original = await api<SystemSettings>("/api/v1/settings/system", {
    token: workspace.token,
  });
  // Face recognition is a global setting and general seeding leaves it off.
  await api<SystemSettings>("/api/v1/settings/system", {
    method: "PATCH",
    token: workspace.token,
    body: JSON.stringify({ ml: { face_enabled: true } }),
  });

  try {
    // Clustering is DBSCAN with a minimum of three faces per person, and the
    // pinned set has two shots of person B, so B's first shot is uploaded
    // twice. Identical pixels produce the identical recognition request.
    const uploads = [
      ...PEOPLE_PERSON_A_ASSETS.map((id, index) => ({ id, person: "a", index })),
      ...[...PEOPLE_PERSON_B_ASSETS, PEOPLE_PERSON_B_ASSETS[0]].map((id, index) => ({
        id,
        person: "b",
        index,
      })),
    ];
    const filenames = uploads.map(
      (upload) => `e2e-people-${upload.person}${upload.index}-${workspace.username}.jpg`,
    );
    const receipts: string[] = [];
    for (const [index, upload] of uploads.entries()) {
      receipts.push(await uploadPortrait(request, workspace, upload.id, filenames[index]!));
    }
    await waitForIngest(workspace.token, receipts);
    const assetIds: string[] = [];
    for (const filename of filenames) {
      assetIds.push(await assetIdByFilename(workspace, filename));
    }
    const expectedA = assetIds.slice(0, PEOPLE_PERSON_A_ASSETS.length).sort();
    const expectedB = assetIds.slice(PEOPLE_PERSON_A_ASSETS.length).sort();

    // The replayed recognition must split the portraits into exactly the two
    // real people before the merge means anything.
    let personA = 0;
    let personB = 0;
    await expect(async () => {
      const people = await peopleInRepository(workspace);
      expect(people).toHaveLength(2);
      const members = await Promise.all(
        people.map(async (person) => ({
          id: person.person_id!,
          assets: await personAssetIds(workspace, person.person_id!),
        })),
      );
      const a = members.find((member) => member.assets.includes(expectedA[0]!));
      const b = members.find((member) => member.assets.includes(expectedB[0]!));
      expect(a?.assets).toEqual(expectedA);
      expect(b?.assets).toEqual(expectedB);
      personA = a!.id;
      personB = b!.id;
    }).toPass({ timeout: PEOPLE_TIMEOUT });

    // A per-attempt name is a data anchor for the merge picker.
    const sourceName = `E2E Person B ${workspace.username}`;
    await api<PersonDetail>(`/api/v1/people/${personB}`, {
      method: "PATCH",
      token: workspace.token,
      body: JSON.stringify({ name: sourceName }),
    });

    await new LoginPage(page).signIn(workspace.username, workspace.password);
    await page.goto(`/people/${personA}`);
    await page.getByRole("button", { name: t("people.details.editAction"), exact: true }).click();
    const dialog = page.getByRole("dialog", { name: t("people.edit.title") });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("tab", { name: t("people.edit.tabs.merge") }).click();
    await dialog.getByPlaceholder(t("people.picker.searchPlaceholder")).fill(sourceName);
    await dialog.getByRole("button", { name: new RegExp(sourceName) }).click();

    const merged = page.waitForResponse(
      (response) =>
        new URL(response.url()).pathname === `/api/v1/people/${personA}/merge` &&
        response.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: t("people.merge.confirm"), exact: true }).click();
    const mergeResponse = await merged;
    expect(mergeResponse.status()).toBe(200);

    // One person remains and owns every face and asset of both.
    const people = await peopleInRepository(workspace);
    expect(people.map((person) => person.person_id)).toEqual([personA]);
    expect(await personAssetIds(workspace, personA)).toEqual([...expectedA, ...expectedB].sort());
    const survivor = await api<PersonDetail>(`/api/v1/people/${personA}`, {
      token: workspace.token,
    });
    expect(survivor.member_count).toBe(uploads.length);
    expect(survivor.asset_count).toBe(uploads.length);

    const mergedAway = await fetch(`${baseURL}/api/v1/people/${personB}`, {
      headers: { authorization: `Bearer ${workspace.token}` },
    });
    await mergedAway.body?.cancel();
    expect(mergedAway.status).toBe(404);
  } finally {
    await api<SystemSettings>("/api/v1/settings/system", {
      method: "PATCH",
      token: workspace.token,
      body: JSON.stringify({ ml: { face_enabled: original.ml?.face_enabled ?? false } }),
    });
  }
});
