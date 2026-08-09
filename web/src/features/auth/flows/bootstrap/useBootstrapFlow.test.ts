import { describe, expect, it } from "vite-plus/test";
import { buildCreateRepositoryRequestBody } from "@/features/repositories";
import { buildBootstrapPrimaryRepositoryRequest } from "./useBootstrapFlow";

describe("bootstrap primary repository request", () => {
  it("submits only the selected immutable storage strategy with the primary identity", () => {
    expect(
      buildCreateRepositoryRequestBody(
        buildBootstrapPrimaryRepositoryRequest("Primary Storage", "cas"),
      ),
    ).toEqual({
      name: "Primary Storage",
      directory_name: undefined,
      root_id: undefined,
      role: "primary",
      storage_strategy: "cas",
      risk_confirmation: undefined,
    });
  });

  it("carries the administrator's storage-risk confirmation during first-run setup", () => {
    expect(
      buildCreateRepositoryRequestBody(
        buildBootstrapPrimaryRepositoryRequest("Primary Storage", "date", true),
      ),
    ).toMatchObject({
      role: "primary",
      storage_strategy: "date",
      risk_confirmation: true,
    });
  });
});
