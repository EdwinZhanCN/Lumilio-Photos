import { describe, expect, it } from "vite-plus/test";
import { buildSetupPrimaryRepositoryRequestBody } from "@/features/repositories";
import { buildBootstrapPrimaryRepositoryRequest } from "./useBootstrapFlow";

describe("bootstrap primary repository request", () => {
  it("submits only the selected immutable storage strategy", () => {
    expect(
      buildSetupPrimaryRepositoryRequestBody(
        buildBootstrapPrimaryRepositoryRequest("Primary Storage", "cas"),
      ),
    ).toEqual({
      name: "Primary Storage",
      storage_strategy: "cas",
      risk_confirmation: undefined,
    });
  });

  it("carries the administrator's storage-risk confirmation during first-run setup", () => {
    expect(
      buildSetupPrimaryRepositoryRequestBody(
        buildBootstrapPrimaryRepositoryRequest("Primary Storage", "date", true),
      ),
    ).toMatchObject({
      storage_strategy: "date",
      risk_confirmation: true,
    });
  });

  it("never carries a role: setup is the only primary creator", () => {
    // `POST /storage/repositories` rejects `role=primary`; the setup endpoint
    // implies it, so the bootstrap request must not send one.
    const body = buildSetupPrimaryRepositoryRequestBody(
      buildBootstrapPrimaryRepositoryRequest("Primary Storage", "date"),
    );
    expect(body).not.toHaveProperty("role");
  });
});
