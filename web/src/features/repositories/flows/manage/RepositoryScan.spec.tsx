import { useState } from "react";
import { describe, expect, it } from "vite-plus/test";
import { http, HttpResponse, worker } from "@test/msw";
import { renderWithProviders } from "@test/render";
import { useRepositoryScan } from "../../api/useRepositoryScan";

const repositoryID = "11111111-1111-1111-1111-111111111111";
const operationID = "22222222-2222-2222-2222-222222222222";

function ScanReceiptProbe() {
  const { scanRepository, isScanning } = useRepositoryScan();
  const [receipt, setReceipt] = useState<string>();

  return (
    <div>
      <button
        type="button"
        onClick={() => {
          void scanRepository(repositoryID).then((result) => setReceipt(result?.operation_id));
        }}
      >
        request scan
      </button>
      <output>{isScanning ? "request pending" : receipt}</output>
    </div>
  );
}

describe("Repository scan operation", () => {
  it("settles the mutation from the durable enqueue receipt without polling for completion", async () => {
    let latestRequests = 0;
    worker.use(
      http.post("*/api/v1/repositories/:id/scan", () =>
        HttpResponse.json({
          operation_id: operationID,
          repository_id: repositoryID,
          mode: "manual",
          status: "queued",
          inserted: true,
          coalesced: false,
        }),
      ),
      http.get("*/api/v1/repositories/:id/scans/latest", () => {
        latestRequests += 1;
        return HttpResponse.json({
          operation_id: operationID,
          repository_id: repositoryID,
          mode: "manual",
          status: "queued",
          started_at: "2026-08-22T00:00:00Z",
        });
      }),
    );

    const screen = await renderWithProviders(<ScanReceiptProbe />);
    await screen.getByRole("button", { name: "request scan", exact: true }).click();

    await expect.element(screen.getByText(operationID, { exact: true })).toBeVisible();
    expect(latestRequests).toBe(0);
  });
});
