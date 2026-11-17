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
      assessment_token: undefined,
      acknowledged_risks: undefined,
    });
  });

  it("binds the administrator's risk acknowledgement to the setup assessment token", () => {
    expect(
      buildSetupPrimaryRepositoryRequestBody(
        buildBootstrapPrimaryRepositoryRequest("Primary Storage", "date", "v1.setup", [
          "removable_storage",
        ]),
      ),
    ).toMatchObject({
      storage_strategy: "date",
      assessment_token: "v1.setup",
      acknowledged_risks: ["removable_storage"],
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
