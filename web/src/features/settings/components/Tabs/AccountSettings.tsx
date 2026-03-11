import { useEffect, useState } from "react";
import { UserCircleIcon } from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import { useUpdateMyProfile } from "@/features/users/hooks/useUsers";
import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];
type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
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

function getInitials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((part) => part[0]?.toUpperCase() ?? "")
    .join("") || "U";
}

export default function AccountSettings() {
  const { t } = useI18n();
  const { user, dispatch } = useAuth();
  const updateProfileMutation = useUpdateMyProfile();
  const [displayName, setDisplayName] = useState("");
  const [avatarURL, setAvatarURL] = useState("");
  const [feedback, setFeedback] = useState<FeedbackState>(null);

  useEffect(() => {
    setDisplayName(user?.display_name ?? "");
    setAvatarURL(user?.avatar_url ?? "");
  }, [user?.display_name, user?.avatar_url]);

  if (!user) {
    return (
      <div className="rounded-2xl border border-base-300 bg-base-200/60 p-6 text-sm opacity-70">
        {t("common.loading", { defaultValue: "Loading..." })}
      </div>
    );
  }

  const resolvedName = user.display_name || user.username || "User";
  const isDirty =
    displayName !== (user.display_name ?? "") || avatarURL !== (user.avatar_url ?? "");

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFeedback(null);

    try {
      const response = await updateProfileMutation.mutateAsync({
        body: {
          display_name: displayName,
          avatar_url: avatarURL,
        },
      });

      const payload = response as ApiResult<Schemas["dto.UserDTO"]> | undefined;
      if (payload?.data) {
        dispatch({ type: "SET_USER", payload: payload.data });
      }

      setFeedback({
        tone: "success",
        message: t("settings.account.saved", {
          defaultValue: "Profile updated successfully.",
        }),
      });
    } catch (error) {
      setFeedback({
        tone: "error",
        message: getErrorMessage(
          error,
          t("settings.account.saveError", {
            defaultValue: "Failed to update profile.",
          }),
        ),
      });
    }
  };

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <UserCircleIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">
          {t("settings.account.title", { defaultValue: "Account" })}
        </h2>
      </div>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,280px)_minmax(0,1fr)]">
        <section className="rounded-3xl border border-base-300 bg-base-200/50 p-6">
          <div className="flex flex-col items-center gap-4 text-center">
            <div className="avatar placeholder">
              <div className="bg-primary text-primary-content size-24 rounded-full text-2xl font-semibold">
                {user.avatar_url ? (
                  <img
                    src={user.avatar_url}
                    alt={resolvedName}
                    className="size-24 rounded-full object-cover"
                  />
                ) : (
                  <span>{getInitials(resolvedName)}</span>
                )}
              </div>
            </div>
            <div className="space-y-1">
              <div className="text-xl font-semibold">{resolvedName}</div>
              <div className="text-sm opacity-70">@{user.username}</div>
              <div className="text-sm opacity-70">{user.email}</div>
            </div>
            <div className="flex flex-wrap justify-center gap-2">
              <span className="badge badge-primary badge-outline">
                {(user.role ?? "user").toUpperCase()}
              </span>
              {(user.permissions ?? []).map((permission) => (
                <span key={permission} className="badge badge-ghost">
                  {permission}
                </span>
              ))}
            </div>
          </div>
        </section>

        <section className="rounded-3xl border border-base-300 bg-base-100 p-6">
          <form className="space-y-5" onSubmit={handleSubmit}>
            {feedback && (
              <div
                className={`alert ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
              >
                <span>{feedback.message}</span>
              </div>
            )}

            <label className="form-control gap-2">
              <span className="font-semibold">
                {t("settings.account.displayName", {
                  defaultValue: "Display name",
                })}
              </span>
              <input
                className="input input-bordered w-full"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                placeholder={user.username ?? "Display name"}
              />
            </label>

            <label className="form-control gap-2">
              <span className="font-semibold">
                {t("settings.account.avatarUrl", {
                  defaultValue: "Avatar URL",
                })}
              </span>
              <input
                className="input input-bordered w-full"
                value={avatarURL}
                onChange={(event) => setAvatarURL(event.target.value)}
                placeholder="https://example.com/avatar.jpg"
              />
              <span className="text-sm opacity-70">
                {t("settings.account.avatarHint", {
                  defaultValue:
                    "Use an image URL. Leave empty to fall back to initials.",
                })}
              </span>
            </label>

            <div className="grid gap-4 md:grid-cols-2">
              <label className="form-control gap-2">
                <span className="font-semibold">
                  {t("settings.account.username", {
                    defaultValue: "Username",
                  })}
                </span>
                <input
                  className="input input-bordered w-full"
                  value={user.username ?? ""}
                  disabled
                />
              </label>

              <label className="form-control gap-2">
                <span className="font-semibold">
                  {t("settings.account.email", {
                    defaultValue: "Email",
                  })}
                </span>
                <input
                  className="input input-bordered w-full"
                  value={user.email ?? ""}
                  disabled
                />
              </label>
            </div>

            <div className="flex justify-end">
              <button
                type="submit"
                className={`btn btn-primary ${updateProfileMutation.isPending ? "loading" : ""}`}
                disabled={!isDirty || updateProfileMutation.isPending}
              >
                {t("settings.account.save", { defaultValue: "Save profile" })}
              </button>
            </div>
          </form>
        </section>
      </div>
    </div>
  );
}
