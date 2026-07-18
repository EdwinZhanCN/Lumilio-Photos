import { useEffect, useMemo, useState } from "react";
import { AlertTriangleIcon, KeyRoundIcon, MoveLeft } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import UserAvatar from "@/components/ui/UserAvatar";
import PhotoPicker from "@/features/assets/picker";
import {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
  useAuth,
} from "@/features/auth";
import {
  useAdminUpdateUser,
  useResetUserAccess,
  useUsers,
  type ManagedUserDTO,
} from "@/features/users";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../SettingsGroup";
import { SettingsDropdown } from "../SettingsDropdown";
import { SettingsSaveBar } from "../SettingsSaveBar";

type UserEditorState = {
  username: string;
  displayName: string;
  avatarAssetId: string;
  role: "admin" | "user";
  isActive: boolean;
};

type FeedbackState = { tone: "success" | "error"; message: string } | null;
type ResetAccessState = {
  temporaryPassword: string;
  clearedPasskeys: boolean;
  clearedTotp: boolean;
} | null;

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  if (error && typeof error === "object") {
    const maybeApiError = error as { message?: string; error?: string };
    if (maybeApiError.message) return maybeApiError.message;
    if (maybeApiError.error) return maybeApiError.error;
  }
  return fallback;
}

function createEditorState(user: ManagedUserDTO): UserEditorState {
  return {
    username: user.username ?? "",
    displayName: user.display_name ?? "",
    avatarAssetId: user.avatar_asset_id ?? "",
    role: user.role === "admin" ? "admin" : "user",
    isActive: user.is_active ?? true,
  };
}

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

export default function UsersTab() {
  const { t } = useI18n();
  const { user, dispatch } = useAuth();
  const usersQuery = useUsers(100, 0);
  const updateUserMutation = useAdminUpdateUser();
  const resetUserAccessMutation = useResetUserAccess();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [form, setForm] = useState<UserEditorState | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [resetAccessState, setResetAccessState] = useState<ResetAccessState>(null);
  const [copiedTemporaryPassword, setCopiedTemporaryPassword] = useState(false);
  const [isChoosingAvatar, setIsChoosingAvatar] = useState(false);

  const users = usersQuery.users;
  const selectedUser = users.find((entry) => entry.user_id === selectedUserId) ?? users[0];
  const isCurrentUser = !!(
    user?.user_id &&
    selectedUser?.user_id &&
    user.user_id === selectedUser.user_id
  );
  const resolvedName = selectedUser
    ? selectedUser.display_name || selectedUser.username || "User"
    : "User";
  const isDirty = useMemo(() => {
    if (!selectedUser || !form) return false;
    const original = createEditorState(selectedUser);
    return (
      form.username !== original.username ||
      form.displayName !== original.displayName ||
      form.avatarAssetId !== original.avatarAssetId ||
      form.role !== original.role ||
      form.isActive !== original.isActive
    );
  }, [form, selectedUser]);

  useEffect(() => {
    if (!selectedUserId && users.length > 0) {
      setSelectedUserId(users[0].user_id ?? null);
    }
  }, [selectedUserId, users]);

  useEffect(() => {
    if (!selectedUser) {
      setForm(null);
      return;
    }
    setForm(createEditorState(selectedUser));
    setFeedback(null);
    setResetAccessState(null);
    setCopiedTemporaryPassword(false);
    setIsChoosingAvatar(false);
  }, [
    selectedUser?.user_id,
    selectedUser?.username,
    selectedUser?.display_name,
    selectedUser?.avatar_asset_id,
    selectedUser?.role,
    selectedUser?.is_active,
  ]);

  if (user?.role !== "admin") {
    return (
      <div className="w-full rounded-2xl bg-base-200/50 p-6 text-sm text-base-content/70">
        {t("settings.users.adminOnly", { defaultValue: "Only administrators can manage users." })}
      </div>
    );
  }

  const handleSave = async () => {
    if (!selectedUser?.user_id || !form) return;
    setFeedback(null);
    try {
      const response = await updateUserMutation.mutateAsync({
        params: { path: { id: selectedUser.user_id } },
        body: {
          username: form.username,
          display_name: form.displayName,
          avatar_asset_id: form.avatarAssetId,
          role: form.role,
          is_active: form.isActive,
        },
      });
      if (response.user_id === user.user_id) {
        dispatch({ type: "SET_USER", payload: response });
      }
      setFeedback({
        tone: "success",
        message: t("settings.users.saved", { defaultValue: "User updated successfully." }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.users.saveError", { defaultValue: "Failed to update user." }),
        ),
      });
    }
  };

  const handleResetAccess = async () => {
    if (!selectedUser?.user_id || isCurrentUser) return;
    setFeedback(null);
    setResetAccessState(null);
    setCopiedTemporaryPassword(false);
    try {
      const response = await resetUserAccessMutation.mutateAsync({
        params: { path: { id: selectedUser.user_id } },
      });
      if (!response.temporary_password) {
        throw new Error("Failed to reset access.");
      }
      setResetAccessState({
        temporaryPassword: response.temporary_password,
        clearedPasskeys: response.cleared_passkeys ?? true,
        clearedTotp: response.cleared_totp ?? true,
      });
      setFeedback({
        tone: "success",
        message: t("settings.users.resetAccessSuccess", {
          defaultValue: "Temporary access issued. Share it securely once.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.users.resetAccessError", { defaultValue: "Failed to reset access." }),
        ),
      });
    }
  };

  const handleCopyTemporaryPassword = async () => {
    if (!resetAccessState?.temporaryPassword) return;
    try {
      await copyToClipboard(resetAccessState.temporaryPassword);
      setCopiedTemporaryPassword(true);
    } catch {
      setCopiedTemporaryPassword(false);
    }
  };

  const effectiveAvatarAssetId = form?.avatarAssetId || undefined;

  const handleAvatarSelect = (assetId: string) => {
    setForm((current) => (current ? { ...current, avatarAssetId: assetId } : current));
    setIsChoosingAvatar(false);
  };

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      {feedback && (
        <div
          className={`rounded-xl px-4 py-3 text-sm ${
            feedback.tone === "success" ? "bg-success/10 text-success" : "bg-error/10 text-error"
          }`}
        >
          {feedback.message}
        </div>
      )}

      <SettingsGroup
        title={t("settings.users.listTitle", { defaultValue: "User directory" })}
        description={t("settings.users.listHint", {
          defaultValue: "Select an account to review profile, role, and access settings.",
        })}
      >
        {usersQuery.isLoading && (
          <SettingsBlock>
            <p className="text-sm text-base-content/60">
              {t("common.loading", { defaultValue: "Loading..." })}
            </p>
          </SettingsBlock>
        )}
        {usersQuery.isError && (
          <SettingsBlock>
            <p className="text-sm text-error">
              {t("settings.users.loadError", { defaultValue: "Failed to load users." })}
            </p>
          </SettingsBlock>
        )}
        {users.map((entry) => (
          <SettingsRow
            key={entry.user_id}
            className={selectedUser?.user_id === entry.user_id ? "bg-primary/8" : ""}
            icon={
              <UserAvatar
                assetId={entry.avatar_asset_id || undefined}
                name={entry.display_name || entry.username || "User"}
                size="size-7"
                textSize="text-xs"
              />
            }
            iconColor="bg-transparent"
            label={entry.display_name || entry.username || "User"}
            description={`@${entry.username} · ${t("settings.users.assetCount", {
              defaultValue: "{{count}} assets",
              count: entry.asset_count ?? 0,
            })} · ${t("settings.users.albumCount", {
              defaultValue: "{{count}} albums",
              count: entry.album_count ?? 0,
            })}`}
            value={
              <span className="flex flex-wrap items-center gap-1.5">
                <span
                  className={`badge badge-sm ${
                    entry.role === "admin" ? "badge-primary badge-outline" : "badge-ghost"
                  }`}
                >
                  {entry.role === "admin" ? "ADMIN" : "USER"}
                </span>
                <span
                  className={`badge badge-sm ${
                    entry.is_active ? "badge-success badge-outline" : "badge-warning badge-outline"
                  }`}
                >
                  {entry.is_active
                    ? t("settings.users.active", { defaultValue: "Active" })
                    : t("settings.users.inactive", { defaultValue: "Inactive" })}
                </span>
              </span>
            }
            onClick={() => setSelectedUserId(entry.user_id ?? null)}
          />
        ))}
      </SettingsGroup>

      {selectedUser && form && (
        <>
          <SettingsGroup
            title={t("settings.users.editTitle", { defaultValue: "Profile Information" })}
            description={t("settings.users.editHint", {
              defaultValue: "Update this user's profile, role, and account status.",
            })}
          >
            <SettingsBlock>
              <div className="flex flex-wrap items-center gap-4">
                <UserAvatar
                  assetId={effectiveAvatarAssetId}
                  name={resolvedName}
                  size="size-16"
                  textSize="text-lg"
                />
                <div className="min-w-0 flex-1">
                  <div className="text-base font-semibold">{resolvedName}</div>
                  <div className="text-sm text-base-content/60">@{selectedUser.username}</div>
                  <div className="mt-1.5 flex flex-wrap gap-3 text-xs text-base-content/55">
                    <span>
                      {t("settings.users.createdAt", { defaultValue: "Created" })}:{" "}
                      {selectedUser.created_at
                        ? new Date(selectedUser.created_at).toLocaleDateString()
                        : t("common.notAvailable", { defaultValue: "N/A" })}
                    </span>
                    <span>
                      {t("settings.users.lastLogin", { defaultValue: "Last login" })}:{" "}
                      {selectedUser.last_login
                        ? new Date(selectedUser.last_login).toLocaleDateString()
                        : t("settings.users.neverLoggedIn", { defaultValue: "Never" })}
                    </span>
                  </div>
                </div>
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    className="btn btn-outline btn-sm"
                    onClick={() => setIsChoosingAvatar(true)}
                  >
                    {effectiveAvatarAssetId
                      ? t("settings.users.changeAvatar", { defaultValue: "Change photo" })
                      : t("settings.users.chooseAvatar", { defaultValue: "Choose photo" })}
                  </button>
                  {effectiveAvatarAssetId && (
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setForm((c) => (c ? { ...c, avatarAssetId: "" } : c))}
                    >
                      {t("settings.users.removeAvatar", { defaultValue: "Remove photo" })}
                    </button>
                  )}
                </div>
              </div>
            </SettingsBlock>

            <SettingsBlock>
              <label htmlFor="user-username" className="text-sm font-medium">
                {t("settings.users.username", { defaultValue: "Username" })}
              </label>
              <input
                id="user-username"
                className="input input-bordered input-sm mt-2 w-full"
                value={form.username}
                onChange={(event) =>
                  setForm((c) =>
                    c ? { ...c, username: normalizeUsernameInput(event.target.value) } : c,
                  )
                }
                pattern={USERNAME_PATTERN}
                minLength={USERNAME_MIN_LENGTH}
                maxLength={USERNAME_MAX_LENGTH}
              />
              <p className="mt-1.5 text-xs text-base-content/55">
                {t("settings.users.usernameHint", { defaultValue: USERNAME_HINT })}
              </p>
            </SettingsBlock>

            <SettingsBlock>
              <label htmlFor="user-display-name" className="text-sm font-medium">
                {t("settings.users.displayName", { defaultValue: "Display name" })}
              </label>
              <input
                id="user-display-name"
                className="input input-bordered input-sm mt-2 w-full"
                value={form.displayName}
                onChange={(event) =>
                  setForm((c) => (c ? { ...c, displayName: event.target.value } : c))
                }
                maxLength={DISPLAY_NAME_MAX_LENGTH}
                placeholder={selectedUser.username ?? "Display name"}
              />
              <p className="mt-1.5 text-xs text-base-content/55">
                {t("settings.users.displayNameHint", { defaultValue: DISPLAY_NAME_HINT })}
              </p>
            </SettingsBlock>

            <SettingsRow
              htmlFor="user-role"
              label={t("settings.users.role", { defaultValue: "Role" })}
              control={
                <SettingsDropdown<"admin" | "user">
                  id="user-role"
                  value={form.role}
                  options={[
                    { value: "user", label: "User" },
                    { value: "admin", label: "Admin" },
                  ]}
                  onChange={(role) => setForm((c) => (c ? { ...c, role } : c))}
                  ariaLabel={t("settings.users.role", { defaultValue: "Role" })}
                  className="w-32"
                />
              }
            />
            <SettingsRow
              htmlFor="user-active"
              label={t("settings.users.status", { defaultValue: "Status" })}
              description={
                form.isActive
                  ? t("settings.users.active", { defaultValue: "Active" })
                  : t("settings.users.inactive", { defaultValue: "Inactive" })
              }
              control={
                <input
                  id="user-active"
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={form.isActive}
                  onChange={(event) =>
                    setForm((c) => (c ? { ...c, isActive: event.target.checked } : c))
                  }
                />
              }
            />
          </SettingsGroup>

          <SettingsGroup
            title={t("settings.users.resetAccessTitle", { defaultValue: "Reset access" })}
            description={
              isCurrentUser
                ? t("settings.users.resetAccessSelfHint", {
                    defaultValue: "Use Change password in Account settings for your own account.",
                  })
                : t("settings.users.resetAccessHint", {
                    defaultValue:
                      "Generate a temporary password and clear every MFA factor for this user.",
                  })
            }
          >
            <SettingsRow
              icon={<KeyRoundIcon className="size-4" />}
              iconColor="bg-warning text-warning-content"
              label={t("settings.users.resetAccess", { defaultValue: "Reset access" })}
              description={
                isCurrentUser
                  ? t("settings.users.resetAccessUnavailable", {
                      defaultValue: "This action is unavailable for your own account.",
                    })
                  : t("settings.users.resetAccessScope", {
                      defaultValue:
                        "Use this when an account owner has lost access to every sign-in factor.",
                    })
              }
              control={
                <button
                  type="button"
                  className={`btn btn-warning btn-outline btn-sm ${resetUserAccessMutation.isPending ? "loading" : ""}`}
                  onClick={() => void handleResetAccess()}
                  disabled={resetUserAccessMutation.isPending || isCurrentUser}
                >
                  {t("settings.users.resetAccess", { defaultValue: "Reset access" })}
                </button>
              }
            />
            {resetAccessState && (
              <SettingsBlock>
                <div className="flex items-start gap-3 rounded-xl bg-warning/10 p-4">
                  <AlertTriangleIcon className="mt-0.5 size-5 shrink-0 text-warning" />
                  <div className="min-w-0 flex-1 space-y-3">
                    <div>
                      <div className="text-sm font-semibold">
                        {t("settings.users.resetAccessPanelTitle", {
                          defaultValue: "Temporary password",
                        })}
                      </div>
                      <p className="mt-1 text-sm text-base-content/70">
                        {t("settings.users.resetAccessPanelBody", {
                          defaultValue:
                            "Share this once through a secure channel. Passkeys, TOTP, and recovery codes were cleared.",
                        })}
                      </p>
                    </div>
                    <div className="flex flex-col gap-3 rounded-lg border border-base-300 bg-base-100 p-3 md:flex-row md:items-center md:justify-between">
                      <code className="break-all text-sm font-semibold">
                        {resetAccessState.temporaryPassword}
                      </code>
                      <button
                        type="button"
                        className="btn btn-sm btn-warning"
                        onClick={() => void handleCopyTemporaryPassword()}
                      >
                        {copiedTemporaryPassword
                          ? t("common.copied", { defaultValue: "Copied" })
                          : t("common.copy", { defaultValue: "Copy" })}
                      </button>
                    </div>
                    <div className="flex flex-wrap gap-2 text-xs">
                      {resetAccessState.clearedPasskeys && (
                        <span className="badge badge-warning badge-outline">
                          {t("settings.users.clearedPasskeys", {
                            defaultValue: "Passkeys cleared",
                          })}
                        </span>
                      )}
                      {resetAccessState.clearedTotp && (
                        <span className="badge badge-warning badge-outline">
                          {t("settings.users.clearedTotp", {
                            defaultValue: "Authenticator App cleared",
                          })}
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              </SettingsBlock>
            )}
          </SettingsGroup>

          <SettingsSaveBar
            isDirty={isDirty}
            isSaving={updateUserMutation.isPending}
            justSaved={feedback?.tone === "success" && updateUserMutation.isSuccess}
            error={updateUserMutation.error}
            canSave={isDirty && !updateUserMutation.isPending}
            onSave={() => void handleSave()}
            onReset={() => setForm(createEditorState(selectedUser))}
            saveLabel={t("settings.users.save", { defaultValue: "Save user" })}
          />
        </>
      )}

      {isChoosingAvatar && selectedUser && (
        <div className="fixed inset-0 z-100 flex flex-col bg-base-100 animate-in slide-in-from-bottom duration-300">
          <div className="sticky top-0 z-10 flex items-center border-b border-base-200 bg-base-100 p-3 shadow-sm">
            <button
              type="button"
              className="btn btn-sm btn-ghost btn-circle"
              onClick={() => setIsChoosingAvatar(false)}
            >
              <MoveLeft size={20} />
            </button>
          </div>
          <div className="flex-1 overflow-hidden">
            <PhotoPicker
              scopeId={`photo-picker:user-avatar:${selectedUser.user_id ?? "unknown"}`}
              onSelect={handleAvatarSelect}
              title={t("settings.users.avatarPhoto", { defaultValue: "Avatar photo" })}
            />
          </div>
        </div>
      )}
    </div>
  );
}
