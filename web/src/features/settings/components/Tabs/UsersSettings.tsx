import { useEffect, useMemo, useState } from "react";
import {
  ExclamationTriangleIcon,
  UsersIcon,
} from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";
import UserAvatar from "@/components/UserAvatar";
import { useAuth } from "@/features/auth";
import {
  useAdminUpdateUser,
  useResetUserAccess,
  useUsers,
  type ManagedUserDTO,
} from "@/features/users/hooks/useUsers";
import {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
  USERNAME_HINT,
  USERNAME_MAX_LENGTH,
  USERNAME_MIN_LENGTH,
  USERNAME_PATTERN,
  normalizeUsernameInput,
} from "@/features/auth/lib/credentialPolicy.ts";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type UserEditorState = {
  username: string;
  displayName: string;
  avatarURL: string;
  role: "admin" | "user";
  isActive: boolean;
};

type FeedbackState = {
  tone: "success" | "error";
  message: string;
} | null;

type ResetAccessState = {
  temporaryPassword: string;
  clearedPasskeys: boolean;
  clearedTotp: boolean;
} | null;

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) {
    return error.message;
  }
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
    avatarURL: user.avatar_url ?? "",
    role: user.role === "admin" ? "admin" : "user",
    isActive: user.is_active ?? true,
  };
}

async function copyToClipboard(value: string) {
  await navigator.clipboard.writeText(value);
}

export default function UsersSettings() {
  const { t } = useI18n();
  const { user, dispatch } = useAuth();
  const usersQuery = useUsers(100, 0);
  const updateUserMutation = useAdminUpdateUser();
  const resetUserAccessMutation = useResetUserAccess();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [form, setForm] = useState<UserEditorState | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState>(null);
  const [resetAccessState, setResetAccessState] =
    useState<ResetAccessState>(null);
  const [copiedTemporaryPassword, setCopiedTemporaryPassword] = useState(false);

  const users = usersQuery.users;
  const selectedUser =
    users.find((entry) => entry.user_id === selectedUserId) ?? users[0];
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
      form.avatarURL !== original.avatarURL ||
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
  }, [
    selectedUser?.user_id,
    selectedUser?.username,
    selectedUser?.display_name,
    selectedUser?.avatar_url,
    selectedUser?.role,
    selectedUser?.is_active,
  ]);

  if (user?.role !== "admin") {
    return (
      <div className="rounded-2xl border border-base-300 bg-base-200/60 p-6 text-sm text-base-content/70">
        {t("settings.users.adminOnly", {
          defaultValue: "Only administrators can manage users.",
        })}
      </div>
    );
  }

  const handleSave = async () => {
    if (!selectedUser?.user_id || !form) {
      return;
    }

    setFeedback(null);

    try {
      const response = await updateUserMutation.mutateAsync({
        params: {
          path: {
            id: selectedUser.user_id,
          },
        },
        body: {
          username: form.username,
          display_name: form.displayName,
          avatar_url: form.avatarURL,
          role: form.role,
          is_active: form.isActive,
        },
      });

      const payload = response as ApiResult<Schemas["dto.UserDTO"]> | undefined;
      if (payload?.data && payload.data.user_id === user.user_id) {
        dispatch({ type: "SET_USER", payload: payload.data });
      }

      setFeedback({
        tone: "success",
        message: t("settings.users.saved", {
          defaultValue: "User updated successfully.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.users.saveError", {
            defaultValue: "Failed to update user.",
          }),
        ),
      });
    }
  };

  const handleResetAccess = async () => {
    if (!selectedUser?.user_id || isCurrentUser) {
      return;
    }

    setFeedback(null);
    setResetAccessState(null);
    setCopiedTemporaryPassword(false);

    try {
      const response = await resetUserAccessMutation.mutateAsync({
        params: {
          path: {
            id: selectedUser.user_id,
          },
        },
      });

      const payload = response as
        | ApiResult<Schemas["dto.ResetAccessResponseDTO"]>
        | undefined;
      const data = payload?.data;
      if (!data?.temporary_password) {
        throw new Error(payload?.message || "Failed to reset access.");
      }

      setResetAccessState({
        temporaryPassword: data.temporary_password,
        clearedPasskeys: data.cleared_passkeys ?? true,
        clearedTotp: data.cleared_totp ?? true,
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
          t("settings.users.resetAccessError", {
            defaultValue: "Failed to reset access.",
          }),
        ),
      });
    }
  };

  const handleCopyTemporaryPassword = async () => {
    if (!resetAccessState?.temporaryPassword) {
      return;
    }

    try {
      await copyToClipboard(resetAccessState.temporaryPassword);
      setCopiedTemporaryPassword(true);
    } catch {
      setCopiedTemporaryPassword(false);
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <UsersIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">
          {t("settings.users.title", { defaultValue: "Users" })}
        </h2>
      </div>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
        <section className="space-y-3">
          {usersQuery.isLoading && (
            <div className="rounded-2xl border border-base-300 bg-base-200/60 p-6 text-sm text-base-content/70">
              {t("common.loading", { defaultValue: "Loading..." })}
            </div>
          )}

          {usersQuery.isError && (
            <div className="rounded-2xl border border-error/30 bg-error/10 p-6 text-sm text-error">
              {t("settings.users.loadError", {
                defaultValue: "Failed to load users.",
              })}
            </div>
          )}

          {users.map((entry) => (
            <button
              key={entry.user_id}
              type="button"
              className={`card w-full border text-left transition ${
                selectedUser?.user_id === entry.user_id
                  ? "border-primary bg-primary/5"
                  : "border-base-300 bg-base-100 hover:border-primary/50"
              }`}
              onClick={() => setSelectedUserId(entry.user_id ?? null)}
            >
              <div className="card-body gap-3">
                <div className="flex items-start justify-between gap-4">
                  <div>
                    <div className="font-semibold text-base-content">
                      {entry.display_name || entry.username || "User"}
                    </div>
                    <div className="text-sm text-base-content/70">
                      @{entry.username}
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <span
                      className={`badge ${
                        entry.role === "admin"
                          ? "badge-primary badge-outline"
                          : "badge-ghost"
                      }`}
                    >
                      {entry.role === "admin" ? "ADMIN" : "USER"}
                    </span>
                    <span
                      className={`badge ${
                        entry.is_active
                          ? "badge-success badge-outline"
                          : "badge-warning badge-outline"
                      }`}
                    >
                      {entry.is_active
                        ? t("settings.users.active", {
                            defaultValue: "Active",
                          })
                        : t("settings.users.inactive", {
                            defaultValue: "Inactive",
                          })}
                    </span>
                  </div>
                </div>

                <div className="flex flex-wrap gap-2 text-xs">
                  <span className="badge badge-ghost">
                    {t("settings.users.assetCount", {
                      defaultValue: "{{count}} assets",
                      count: entry.asset_count ?? 0,
                    })}
                  </span>
                  <span className="badge badge-ghost">
                    {t("settings.users.albumCount", {
                      defaultValue: "{{count}} albums",
                      count: entry.album_count ?? 0,
                    })}
                  </span>
                </div>
              </div>
            </button>
          ))}
        </section>

        {/* ── Editor: Avatar sidebar + form ───────────────────────── */}
        {!selectedUser || !form ? (
          <div className="rounded-3xl border border-base-300 bg-base-100 p-6 text-sm text-base-content/70">
            {t("settings.users.empty", {
              defaultValue: "Select a user to edit.",
            })}
          </div>
        ) : (
          <div className="flex flex-col gap-6">
            {/* Feedback + reset access alerts (above the grid) */}
            {feedback && (
              <div
                className={`alert ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
              >
                <span>{feedback.message}</span>
              </div>
            )}

            {resetAccessState && (
              <div className="rounded-3xl border border-warning/30 bg-warning/10 p-5">
                <div className="flex items-start gap-3">
                  <ExclamationTriangleIcon className="mt-0.5 size-5 shrink-0 text-warning" />
                  <div className="min-w-0 flex-1 space-y-3">
                    <div>
                      <div className="font-semibold text-warning-content">
                        {t("settings.users.resetAccessPanelTitle", {
                          defaultValue: "Temporary password",
                        })}
                      </div>
                      <p className="mt-1 text-sm text-base-content/80">
                        {t("settings.users.resetAccessPanelBody", {
                          defaultValue:
                            "Share this once through a secure channel. Passkeys, TOTP, and recovery codes were cleared.",
                        })}
                      </p>
                    </div>

                    <div className="flex flex-col gap-3 rounded-2xl border border-base-300 bg-base-100 p-4 md:flex-row md:items-center md:justify-between">
                      <code className="break-all text-sm font-semibold text-base-content">
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
              </div>
            )}

            {/* Avatar card + Profile form grid */}
            <div className="grid gap-6 lg:grid-cols-[minmax(0,260px)_minmax(0,1fr)]">
              {/* Avatar card */}
              <div className="rounded-3xl border border-base-300 bg-base-100 p-6 shadow-sm">
                <div className="flex flex-col items-center gap-4 text-center">
                  <UserAvatar
                    src={selectedUser.avatar_url}
                    name={resolvedName}
                  />

                  <div className="space-y-0.5">
                    <div className="text-lg font-semibold text-base-content">
                      {resolvedName}
                    </div>
                    <div className="text-sm text-base-content/60">
                      @{selectedUser.username}
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center justify-center gap-2">
                    <span
                      className={`badge font-medium ${
                        selectedUser.role === "admin"
                          ? "badge-primary badge-outline"
                          : "badge-ghost"
                      }`}
                    >
                      {(selectedUser.role ?? "user").toUpperCase()}
                    </span>
                    <span
                      className={`badge font-medium ${
                        selectedUser.is_active
                          ? "badge-success badge-outline"
                          : "badge-warning badge-outline"
                      }`}
                    >
                      {selectedUser.is_active
                        ? t("settings.users.active", {
                            defaultValue: "Active",
                          })
                        : t("settings.users.inactive", {
                            defaultValue: "Inactive",
                          })}
                    </span>
                  </div>

                  {/* Metadata */}
                  <div className="w-full space-y-2 border-t border-base-300 pt-4 text-xs text-base-content/60">
                    <div className="flex justify-between">
                      <span>
                        {t("settings.users.createdAt", {
                          defaultValue: "Created",
                        })}
                      </span>
                      <span className="text-base-content/80">
                        {selectedUser.created_at
                          ? new Date(
                              selectedUser.created_at,
                            ).toLocaleDateString()
                          : t("common.notAvailable", {
                              defaultValue: "N/A",
                            })}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>
                        {t("settings.users.lastLogin", {
                          defaultValue: "Last login",
                        })}
                      </span>
                      <span className="text-base-content/80">
                        {selectedUser.last_login
                          ? new Date(
                              selectedUser.last_login,
                            ).toLocaleDateString()
                          : t("settings.users.neverLoggedIn", {
                              defaultValue: "Never",
                            })}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>
                        {t("settings.users.assetCount", {
                          defaultValue: "Assets",
                          count: selectedUser.asset_count ?? 0,
                        })}
                      </span>
                      <span className="text-base-content/80">
                        {selectedUser.asset_count ?? 0}
                      </span>
                    </div>
                    <div className="flex justify-between">
                      <span>
                        {t("settings.users.albumCount", {
                          defaultValue: "Albums",
                          count: selectedUser.album_count ?? 0,
                        })}
                      </span>
                      <span className="text-base-content/80">
                        {selectedUser.album_count ?? 0}
                      </span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Profile form */}
              <div className="rounded-3xl border border-base-300 bg-base-100 p-6 shadow-sm">
                <h3 className="text-lg font-semibold text-base-content">
                  {t("settings.users.editTitle", {
                    defaultValue: "Profile Information",
                  })}
                </h3>
                <p className="mt-1 text-sm text-base-content/60">
                  {t("settings.users.editHint", {
                    defaultValue:
                      "Update this user's profile, role, and account status.",
                  })}
                </p>

                <div className="mt-5 space-y-5">
                  {/* Username */}
                  <fieldset className="fieldset">
                    <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                      {t("settings.users.username", {
                        defaultValue: "Username",
                      })}
                    </legend>
                    <input
                      className="input input-bordered w-full bg-base-100"
                      value={form.username}
                      onChange={(event) =>
                        setForm((current) =>
                          current
                            ? {
                                ...current,
                                username: normalizeUsernameInput(
                                  event.target.value,
                                ),
                              }
                            : current,
                        )
                      }
                      pattern={USERNAME_PATTERN}
                      minLength={USERNAME_MIN_LENGTH}
                      maxLength={USERNAME_MAX_LENGTH}
                    />
                    <p className="mt-1 text-xs text-base-content/60">
                      {t("settings.users.usernameHint", {
                        defaultValue: USERNAME_HINT,
                      })}
                    </p>
                  </fieldset>

                  {/* Display name */}
                  <fieldset className="fieldset">
                    <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                      {t("settings.users.displayName", {
                        defaultValue: "Display name",
                      })}
                    </legend>
                    <input
                      className="input input-bordered w-full bg-base-100"
                      value={form.displayName}
                      onChange={(event) =>
                        setForm((current) =>
                          current
                            ? { ...current, displayName: event.target.value }
                            : current,
                        )
                      }
                      maxLength={DISPLAY_NAME_MAX_LENGTH}
                      placeholder={selectedUser.username ?? "Display name"}
                    />
                    <p className="mt-1 text-xs text-base-content/60">
                      {t("settings.users.displayNameHint", {
                        defaultValue: DISPLAY_NAME_HINT,
                      })}
                    </p>
                  </fieldset>

                  {/* Avatar URL */}
                  <fieldset className="fieldset">
                    <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                      {t("settings.users.avatarUrl", {
                        defaultValue: "Avatar URL",
                      })}
                    </legend>
                    <input
                      className="input input-bordered w-full bg-base-100"
                      value={form.avatarURL}
                      onChange={(event) =>
                        setForm((current) =>
                          current
                            ? { ...current, avatarURL: event.target.value }
                            : current,
                        )
                      }
                      placeholder="https://example.com/avatar.jpg"
                    />
                    <p className="mt-1 text-xs text-base-content/60">
                      {t("settings.users.avatarUrlHint", {
                        defaultValue:
                          "Use an image URL. Leave empty to fall back to initials.",
                      })}
                    </p>
                  </fieldset>

                  {/* Role + Status row */}
                  <div className="grid gap-4 sm:grid-cols-2">
                    <fieldset className="fieldset">
                      <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                        {t("settings.users.role", {
                          defaultValue: "Role",
                        })}
                      </legend>
                      <select
                        className="select select-bordered w-full bg-base-100"
                        value={form.role}
                        onChange={(event) =>
                          setForm((current) =>
                            current
                              ? {
                                  ...current,
                                  role:
                                    event.target.value === "admin"
                                      ? "admin"
                                      : "user",
                                }
                              : current,
                          )
                        }
                      >
                        <option value="user">User</option>
                        <option value="admin">Admin</option>
                      </select>
                    </fieldset>

                    <fieldset className="fieldset">
                      <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                        {t("settings.users.status", {
                          defaultValue: "Status",
                        })}
                      </legend>
                      <label className="label cursor-pointer justify-start gap-3 rounded-xl border border-base-300 bg-base-100 px-4 py-3">
                        <input
                          type="checkbox"
                          className="toggle toggle-primary"
                          checked={form.isActive}
                          onChange={(event) =>
                            setForm((current) =>
                              current
                                ? {
                                    ...current,
                                    isActive: event.target.checked,
                                  }
                                : current,
                            )
                          }
                        />
                        <span className="text-base-content">
                          {form.isActive
                            ? t("settings.users.active", {
                                defaultValue: "Active",
                              })
                            : t("settings.users.inactive", {
                                defaultValue: "Inactive",
                              })}
                        </span>
                      </label>
                    </fieldset>
                  </div>

                  {/* Save */}
                  <div className="flex justify-end pt-1">
                    <button
                      type="button"
                      className={`btn btn-primary ${updateUserMutation.isPending ? "loading" : ""}`}
                      onClick={() => void handleSave()}
                      disabled={!isDirty || updateUserMutation.isPending}
                    >
                      {t("settings.users.save", {
                        defaultValue: "Save user",
                      })}
                    </button>
                  </div>
                </div>
              </div>
            </div>

            {/* ── Reset access section ──────────────────────────────── */}
            <div className="flex flex-col gap-3 rounded-3xl border border-base-300 bg-base-100 p-6 shadow-sm md:flex-row md:items-center md:justify-between">
              <div className="space-y-1">
                <div className="font-semibold text-base-content">
                  {t("settings.users.resetAccessTitle", {
                    defaultValue: "Reset access",
                  })}
                </div>
                <p className="text-sm text-base-content/70">
                  {isCurrentUser
                    ? t("settings.users.resetAccessSelfHint", {
                        defaultValue:
                          "Use Change password in Account settings for your own account.",
                      })
                    : t("settings.users.resetAccessHint", {
                        defaultValue:
                          "Generate a temporary password and clear every MFA factor for this user.",
                      })}
                </p>
              </div>

              <button
                type="button"
                className={`btn btn-warning btn-outline shrink-0 ${resetUserAccessMutation.isPending ? "loading" : ""}`}
                onClick={() => void handleResetAccess()}
                disabled={resetUserAccessMutation.isPending || isCurrentUser}
              >
                {t("settings.users.resetAccess", {
                  defaultValue: "Reset access",
                })}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
