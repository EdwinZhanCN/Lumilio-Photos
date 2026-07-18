import { describe, expect, it } from "vite-plus/test";
import {
  areRequiredCloudFieldsFilled,
  createCloudCredentialFormValues,
} from "./cloudCredentialForm";

const fields = [
  { name: "endpoint", type: "text", required: true },
  {
    name: "region",
    type: "select",
    required: true,
    options: [{ value: "auto", label: "Automatic" }],
  },
];

describe("cloud credential form model", () => {
  it("starts selects at their first option and text fields empty", () => {
    expect(createCloudCredentialFormValues(fields)).toEqual({
      endpoint: "",
      region: "auto",
    });
  });

  it("requires every required field to contain a non-whitespace value", () => {
    expect(
      areRequiredCloudFieldsFilled(fields, { endpoint: " https://example.test ", region: "auto" }),
    ).toBe(true);
    expect(areRequiredCloudFieldsFilled(fields, { endpoint: " ", region: "auto" })).toBe(false);
  });
});
