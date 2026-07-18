import { describe, expect, it } from "vite-plus/test";
import { createUserEditorState } from "./userEditor";

describe("createUserEditorState", () => {
  it("normalizes optional values and unknown roles for the local editor", () => {
    expect(
      createUserEditorState({
        user_id: 7,
        username: "alex",
        role: "owner",
        is_active: false,
      }),
    ).toEqual({
      username: "alex",
      displayName: "",
      avatarAssetId: "",
      role: "user",
      isActive: false,
    });
  });
});
