import type { ManagedUserDTO } from "@/features/users";

export type UserEditorState = {
  username: string;
  displayName: string;
  avatarAssetId: string;
  role: "admin" | "user";
  isActive: boolean;
};

export type ResetAccessState = {
  temporaryPassword: string;
  clearedPasskeys: boolean;
  clearedTotp: boolean;
} | null;

export function createUserEditorState(user: ManagedUserDTO): UserEditorState {
  return {
    username: user.username ?? "",
    displayName: user.display_name ?? "",
    avatarAssetId: user.avatar_asset_id ?? "",
    role: user.role === "admin" ? "admin" : "user",
    isActive: user.is_active ?? true,
  };
}
