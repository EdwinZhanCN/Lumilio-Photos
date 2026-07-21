import { describe, expect, expectTypeOf, it } from "vite-plus/test";
import type { components, paths } from "./schema.d.ts";

type LoginBody = paths["/api/v1/auth/login"]["post"]["requestBody"]["content"]["application/json"];
type CurrentUserResponse =
  paths["/api/v1/auth/me"]["get"]["responses"][200]["content"]["application/json"];

describe("generated OpenAPI contract", () => {
  it("keeps required JSON bodies exact and non-empty", () => {
    expectTypeOf<LoginBody>().toEqualTypeOf<components["schemas"]["dto.LoginRequestDTO"]>();

    const valid: LoginBody = { username: "alice", password: "secret" };
    expect(valid.username).toBe("alice");

    // @ts-expect-error required request bodies must not accept an empty object
    const missing: LoginBody = {};
    // @ts-expect-error generated request bodies must reject unknown fields
    const unknown: LoginBody = { username: "alice", password: "secret", admin: true };
    expect([missing, unknown]).toHaveLength(2);
  });

  it("preserves known response payload types", () => {
    expectTypeOf<CurrentUserResponse>().toEqualTypeOf<components["schemas"]["dto.UserDTO"]>();
  });
});
