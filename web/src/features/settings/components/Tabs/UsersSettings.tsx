import { useEffect, useState } from "react";
import { ShieldCheckIcon, UsersIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import {
  useAdminUpdateUser,
  useUsers,
  type ManagedUserDTO,
} from "@/features/users/hooks/useUsers";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

type UserEditorState = {
  username: string;
  displayName: string;
  email: string;
  avatarURL: string;
  role: "admin" | "user";
  isActive: boolean;
};

type FeedbackState = {
  tone: "success" | "error";
  message: string;
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
    email: user.email ?? "",
    avatarURL: user.avatar_url ?? "",
    role: user.role === "admin" ? "admin" : "user",
    isActive: user.is_active ?? true,
  };
}

export default function UsersSettings() {
  const { t } = useI18n();
  const { user, dispatch } = useAuth();
  const usersQuery = useUsers(100, 0);
  const updateUserMutation = useAdminUpdateUser();
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null);
  const [form, setForm] = useState<UserEditorState | null>(null);
  const [feedback, setFeedback] = useState<FeedbackState>(null);

  const users = usersQuery.users;
  const selectedUser =
    users.find((entry) => entry.user_id === selectedUserId) ?? users[0];

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
  }, [
    selectedUser?.user_id,
    selectedUser?.username,
    selectedUser?.display_name,
    selectedUser?.email,
    selectedUser?.avatar_url,
    selectedUser?.role,
    selectedUser?.is_active,
  ]);

  if (user?.role !== "admin") {
    return (
      <div className="rounded-2xl border border-base-300 bg-base-200/60 p-6 text-sm opacity-70">
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
          email: form.email,
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
            <div className="rounded-2xl border border-base-300 bg-base-200/60 p-6 text-sm opacity-70">
              {t("common.loading", { defaultValue: "Loading..." })}
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
                    <div className="font-semibold">
                      {entry.display_name || entry.username || "User"}
                    </div>
                    <div className="text-sm opacity-70">@{entry.username}</div>
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

                <div className="text-sm opacity-70">{entry.email}</div>

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

        <section className="rounded-3xl border border-base-300 bg-base-100 p-6">
          {!selectedUser || !form ? (
            <div className="text-sm opacity-70">
              {t("settings.users.empty", {
                defaultValue: "Select a user to edit.",
              })}
            </div>
          ) : (
            <div className="space-y-5">
              {feedback && (
                <div
                  className={`alert ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
                >
                  <span>{feedback.message}</span>
                </div>
              )}

              <div className="flex items-center gap-2">
                <ShieldCheckIcon className="size-5 text-primary" />
                <div>
                  <div className="font-semibold">
                    {selectedUser.display_name || selectedUser.username}
                  </div>
                  <div className="text-sm opacity-70">
                    ID {selectedUser.user_id}
                  </div>
                </div>
              </div>

              <div className="grid gap-4 md:grid-cols-2">
                <label className="form-control gap-2">
                  <span className="font-semibold">
                    {t("settings.users.username", {
                      defaultValue: "Username",
                    })}
                  </span>
                  <input
                    className="input input-bordered"
                    value={form.username}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? { ...current, username: event.target.value }
                          : current,
                      )
                    }
                  />
                </label>

                <label className="form-control gap-2">
                  <span className="font-semibold">
                    {t("settings.users.displayName", {
                      defaultValue: "Display name",
                    })}
                  </span>
                  <input
                    className="input input-bordered"
                    value={form.displayName}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? { ...current, displayName: event.target.value }
                          : current,
                      )
                    }
                  />
                </label>

                <label className="form-control gap-2 md:col-span-2">
                  <span className="font-semibold">
                    {t("settings.users.email", {
                      defaultValue: "Email",
                    })}
                  </span>
                  <input
                    className="input input-bordered"
                    value={form.email}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? { ...current, email: event.target.value }
                          : current,
                      )
                    }
                  />
                </label>

                <label className="form-control gap-2 md:col-span-2">
                  <span className="font-semibold">
                    {t("settings.users.avatarUrl", {
                      defaultValue: "Avatar URL",
                    })}
                  </span>
                  <input
                    className="input input-bordered"
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
                </label>

                <label className="form-control gap-2">
                  <span className="font-semibold">
                    {t("settings.users.role", {
                      defaultValue: "Role",
                    })}
                  </span>
                  <select
                    className="select select-bordered"
                    value={form.role}
                    onChange={(event) =>
                      setForm((current) =>
                        current
                          ? {
                              ...current,
                              role:
                                event.target.value === "admin" ? "admin" : "user",
                            }
                          : current,
                      )
                    }
                  >
                    <option value="user">User</option>
                    <option value="admin">Admin</option>
                  </select>
                </label>

                <label className="form-control gap-2">
                  <span className="font-semibold">
                    {t("settings.users.status", {
                      defaultValue: "Status",
                    })}
                  </span>
                  <label className="label cursor-pointer justify-start gap-3 rounded-2xl border border-base-300 px-4 py-3">
                    <input
                      type="checkbox"
                      className="toggle toggle-primary"
                      checked={form.isActive}
                      onChange={(event) =>
                        setForm((current) =>
                          current
                            ? { ...current, isActive: event.target.checked }
                            : current,
                        )
                      }
                    />
                    <span>
                      {form.isActive
                        ? t("settings.users.active", {
                            defaultValue: "Active",
                          })
                        : t("settings.users.inactive", {
                            defaultValue: "Inactive",
                          })}
                    </span>
                  </label>
                </label>
              </div>

              <div className="flex justify-end">
                <button
                  type="button"
                  className={`btn btn-primary ${updateUserMutation.isPending ? "loading" : ""}`}
                  onClick={() => void handleSave()}
                  disabled={updateUserMutation.isPending}
                >
                  {t("settings.users.save", { defaultValue: "Save user" })}
                </button>
              </div>
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
