import { useEffect, useMemo, useState } from "react";
import {
  Check,
  ChevronRight,
  Fingerprint,
  KeyRound,
  Plus,
  ShieldCheckIcon,
  Trash2,
  UserCircleIcon,
  X,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useAuth } from "@/features/auth";
import UserAvatar from "@/components/UserAvatar";
import { useMFAStatus } from "@/features/auth/hooks/useMFA.ts";
import {
  useBeginPasskeyEnrollment,
  useDeletePasskey,
  usePasskeys,
  useVerifyPasskeyEnrollment,
} from "@/features/auth/hooks/usePasskeys.ts";
import {
  createPasskeyCredential,
  getPasskeySupport,
} from "@/features/auth/lib/webauthn.ts";
import {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
} from "@/features/auth/lib/credentialPolicy.ts";
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

/* ------------------------------------------------------------------ */
/*  Permission display config                                          */
/* ------------------------------------------------------------------ */

const ALL_KNOWN_PERMISSIONS: {
  key: string;
  label: string;
  description: string;
}[] = [
  {
    key: "manage_users",
    label: "Manage Users",
    description: "Create, edit, and delete user accounts",
  },
  {
    key: "manage_assets",
    label: "Manage Assets",
    description: "Upload, edit, and delete all assets",
  },
  {
    key: "manage_albums",
    label: "Manage Albums",
    description: "Create, edit, and delete albums and collections",
  },
  {
    key: "manage_settings",
    label: "Manage Settings",
    description: "Modify server and application settings",
  },
  {
    key: "view_assets",
    label: "View Assets",
    description: "Browse and view assets in the library",
  },
  {
    key: "upload_assets",
    label: "Upload Assets",
    description: "Upload new photos, videos, and files",
  },
  {
    key: "manage_plugins",
    label: "Manage Plugins",
    description: "Install, configure, and remove plugins",
  },
  {
    key: "manage_server",
    label: "Manage Server",
    description: "Access server monitoring and administration",
  },
];

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

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

/* ------------------------------------------------------------------ */
/*  Sub-components                                                     */
/* ------------------------------------------------------------------ */

function PermissionsModal({
  permissions,
  role,
  t,
}: {
  permissions: string[];
  role: string;
  t: ReturnType<typeof useI18n>["t"];
}) {
  const permissionSet = useMemo(() => new Set(permissions), [permissions]);

  // Include any permissions the user has that aren't in our known list
  const extraPermissions = useMemo(
    () =>
      permissions.filter(
        (p) => !ALL_KNOWN_PERMISSIONS.some((k) => k.key === p),
      ),
    [permissions],
  );

  return (
    <dialog
      id="permissions_modal"
      className="modal modal-bottom sm:modal-middle"
    >
      <div className="modal-box max-w-lg">
        <form method="dialog">
          <button className="btn btn-sm btn-circle btn-ghost absolute right-3 top-3">
            <X className="size-4" />
          </button>
        </form>

        <h3 className="text-lg font-bold">
          {t("settings.account.permissionsTitle", {
            defaultValue: "Account Permissions",
          })}
        </h3>
        <p className="mt-1 text-sm text-base-content/70">
          {t("settings.account.permissionsSubtitle", {
            defaultValue:
              "Permissions assigned to your account based on your role.",
          })}
        </p>

        <div className="mt-1 mb-4">
          <span className="badge badge-primary badge-sm">
            {role.toUpperCase()}
          </span>
        </div>

        <div className="divide-y divide-base-200">
          {ALL_KNOWN_PERMISSIONS.map((perm) => {
            const granted = permissionSet.has(perm.key);
            return (
              <div key={perm.key} className="flex items-center gap-3 py-3">
                <div
                  className={`flex size-7 shrink-0 items-center justify-center rounded-full ${
                    granted
                      ? "bg-success/15 text-success"
                      : "bg-base-200 text-base-content/30"
                  }`}
                >
                  {granted ? (
                    <Check className="size-4" strokeWidth={2.5} />
                  ) : (
                    <X className="size-4" strokeWidth={2.5} />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <div
                    className={`text-sm font-medium ${
                      granted ? "text-base-content" : "text-base-content/50"
                    }`}
                  >
                    {perm.label}
                  </div>
                  <div className="text-xs text-base-content/60">
                    {perm.description}
                  </div>
                </div>
              </div>
            );
          })}

          {/* Render any extra permissions not in the known list */}
          {extraPermissions.map((perm) => (
            <div key={perm} className="flex items-center gap-3 py-3">
              <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-success/15 text-success">
                <Check className="size-4" strokeWidth={2.5} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-base-content">
                  {perm
                    .replace(/_/g, " ")
                    .replace(/\b\w/g, (c) => c.toUpperCase())}
                </div>
                <div className="text-xs text-base-content/60">
                  Custom permission
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="modal-action">
          <form method="dialog">
            <button className="btn btn-sm">
              {t("common.close", { defaultValue: "Close" })}
            </button>
          </form>
        </div>
      </div>
      <form method="dialog" className="modal-backdrop">
        <button>close</button>
      </form>
    </dialog>
  );
}

function SecurityLinkCard({
  icon,
  title,
  description,
  badge,
  badgeClass,
  onClick,
}: {
  icon: React.ReactNode;
  title: string;
  description: string;
  badge?: string;
  badgeClass?: string;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      className="flex w-full items-center gap-4 rounded-2xl border border-base-300 bg-base-100 p-5 text-left shadow-sm transition-colors hover:bg-base-200/60"
      onClick={onClick}
    >
      <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
        {icon}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2.5">
          <span className="font-semibold text-base-content">{title}</span>
          {badge && (
            <span className={`badge badge-sm ${badgeClass ?? "badge-ghost"}`}>
              {badge}
            </span>
          )}
        </div>
        <p className="mt-0.5 text-sm text-base-content/70">{description}</p>
      </div>
      <ChevronRight className="size-5 shrink-0 text-base-content/40" />
    </button>
  );
}

function PasskeysModal({
  passkeys,
  passkeysLoading,
  passkeySupport,
  passkeyBusy,
  deleteIsPending,
  onAdd,
  onDelete,
  feedback,
  t,
}: {
  passkeys: Schemas["dto.PasskeyCredentialSummaryDTO"][];
  passkeysLoading: boolean;
  passkeySupport: {
    supported: boolean;
    reasonKey?:
      | "auth.passkeySupport.browserOnly"
      | "auth.passkeySupport.notSupported"
      | "auth.passkeySupport.secureContextRequired"
      | "auth.passkeySupport.httpsRequired";
  };
  passkeyBusy: boolean;
  deleteIsPending: boolean;
  onAdd: () => void;
  onDelete: (id: number) => void;
  feedback: FeedbackState;
  t: ReturnType<typeof useI18n>["t"];
}) {
  return (
    <dialog id="passkeys_modal" className="modal modal-bottom sm:modal-middle">
      <div className="modal-box max-w-lg">
        <form method="dialog">
          <button className="btn btn-sm btn-circle btn-ghost absolute right-3 top-3">
            <X className="size-4" />
          </button>
        </form>

        {/* Header */}
        <div className="flex items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Fingerprint className="size-5" />
          </div>
          <div>
            <h3 className="text-lg font-bold">
              {t("settings.account.mfa.enrolledPasskeys", {
                defaultValue: "Passkeys",
              })}
            </h3>
            <p className="text-sm text-base-content/60">
              {t("settings.account.mfa.enrolledPasskeysHint", {
                defaultValue:
                  "Manage the passkey credentials registered to this account.",
              })}
            </p>
          </div>
        </div>

        {/* Feedback */}
        {feedback && (
          <div
            className={`alert mt-4 ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
          >
            <span>{feedback.message}</span>
          </div>
        )}

        {/* Passkey list */}
        <div className="mt-5 space-y-2.5">
          {passkeysLoading ? (
            <div className="flex items-center justify-center gap-3 rounded-2xl border border-base-300 bg-base-200/50 px-5 py-6">
              <span className="loading loading-spinner loading-sm" />
              <span className="text-sm text-base-content/70">
                {t("common.loading", { defaultValue: "Loading..." })}
              </span>
            </div>
          ) : passkeys.length === 0 ? (
            <div className="flex flex-col items-center gap-3 rounded-2xl border border-dashed border-base-300 bg-base-200/20 px-5 py-8 text-center">
              <Fingerprint className="size-8 text-base-content/30" />
              <div>
                <p className="text-sm font-medium text-base-content/60">
                  {t("settings.account.mfa.noPasskeys", {
                    defaultValue: "No passkeys enrolled yet",
                  })}
                </p>
                <p className="mt-0.5 text-xs text-base-content/50">
                  {passkeySupport.supported
                    ? t("settings.account.mfa.addPasskeyPrompt", {
                        defaultValue:
                          "Add a passkey to sign in with your device's biometrics or security key.",
                      })
                    : passkeySupport.reasonKey
                      ? t(passkeySupport.reasonKey)
                      : ""}
                </p>
              </div>
            </div>
          ) : (
            passkeys.map((passkey) => (
              <div
                key={passkey.passkey_id}
                className="flex items-center justify-between gap-3 rounded-2xl border border-base-300 bg-base-200/30 px-4 py-3"
              >
                <div className="min-w-0">
                  <div className="text-sm font-semibold text-base-content">
                    {passkey.label}
                  </div>
                  <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-base-content/55">
                    <span>
                      {t("settings.account.mfa.passkeyCreated", {
                        defaultValue: "Created",
                      })}{" "}
                      {new Date(passkey.created_at ?? "").toLocaleDateString()}
                    </span>
                    {passkey.last_used_at && (
                      <span>
                        {t("settings.account.mfa.passkeyLastUsed", {
                          defaultValue: "Last used",
                        })}{" "}
                        {new Date(passkey.last_used_at).toLocaleDateString()}
                      </span>
                    )}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm btn-square text-error"
                  disabled={deleteIsPending}
                  onClick={() => onDelete(passkey.passkey_id ?? 0)}
                  title={t("common.remove", { defaultValue: "Remove" })}
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            ))
          )}
        </div>

        {/* Actions */}
        <div className="modal-action">
          <button
            type="button"
            className={`btn btn-primary btn-sm gap-1.5 ${passkeyBusy ? "loading" : ""}`}
            disabled={!passkeySupport.supported || passkeyBusy}
            onClick={onAdd}
          >
            <Plus className="size-4" />
            {t("settings.account.mfa.addPasskey", {
              defaultValue: "Add Passkey",
            })}
          </button>
          <form method="dialog">
            <button className="btn btn-ghost btn-sm">
              {t("common.close", { defaultValue: "Close" })}
            </button>
          </form>
        </div>
      </div>
      <form method="dialog" className="modal-backdrop">
        <button>close</button>
      </form>
    </dialog>
  );
}

/* ------------------------------------------------------------------ */
/*  Main component                                                     */
/* ------------------------------------------------------------------ */

export default function AccountSettings() {
  const { t } = useI18n();
  const { user, dispatch } = useAuth();
  const navigate = useNavigate();
  const mfaStatusQuery = useMFAStatus();
  const passkeysQuery = usePasskeys();
  const updateProfileMutation = useUpdateMyProfile();
  const beginPasskeyEnrollment = useBeginPasskeyEnrollment();
  const verifyPasskeyEnrollment = useVerifyPasskeyEnrollment();
  const deletePasskeyMutation = useDeletePasskey();

  const [displayName, setDisplayName] = useState("");
  const [avatarURL, setAvatarURL] = useState("");
  const [profileFeedback, setProfileFeedback] = useState<FeedbackState>(null);
  const [securityFeedback, setSecurityFeedback] = useState<FeedbackState>(null);
  const passkeySupport = useMemo(() => getPasskeySupport(), []);

  const openPasskeysModal = () => {
    const modal = document.getElementById(
      "passkeys_modal",
    ) as HTMLDialogElement | null;
    modal?.showModal();
  };

  useEffect(() => {
    setDisplayName(user?.display_name ?? "");
    setAvatarURL(user?.avatar_url ?? "");
  }, [user?.display_name, user?.avatar_url]);

  if (!user) {
    return (
      <div className="flex items-center gap-3 rounded-2xl border border-base-300 bg-base-200/50 px-5 py-4">
        <span className="loading loading-spinner loading-sm" />
        <span className="text-sm text-base-content/70">
          {t("common.loading", { defaultValue: "Loading..." })}
        </span>
      </div>
    );
  }

  const resolvedName = user.display_name || user.username || "User";
  const mfaStatus = mfaStatusQuery.data?.data;
  const isDirty =
    displayName !== (user.display_name ?? "") ||
    avatarURL !== (user.avatar_url ?? "");
  const passkeys = passkeysQuery.passkeys;
  const passkeyBusy =
    beginPasskeyEnrollment.isPending ||
    verifyPasskeyEnrollment.isPending ||
    deletePasskeyMutation.isPending;

  /* ---- Handlers ---- */

  const handleMFAToggle = () => {
    const search = mfaStatus?.totp_enabled ? "?action=disable" : "?mfa=setup";
    navigate(`/mfa${search}`, {
      state: {
        from: {
          pathname: "/settings",
          search: "?tab=account",
        },
      },
    });
  };

  const handleProfileSubmit = async (
    event: React.FormEvent<HTMLFormElement>,
  ) => {
    event.preventDefault();
    setProfileFeedback(null);

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

      setProfileFeedback({
        tone: "success",
        message: t("settings.account.saved", {
          defaultValue: "Profile updated successfully.",
        }),
      });
    } catch (error) {
      setProfileFeedback({
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

  const handleAddPasskey = async () => {
    setSecurityFeedback(null);

    try {
      const optionsResponse = await beginPasskeyEnrollment.mutateAsync({});
      const optionsPayload = optionsResponse as
        | ApiResult<Schemas["dto.PasskeyOptionsResponseDTO"]>
        | undefined;
      if (!optionsPayload?.data) {
        throw new Error(
          optionsPayload?.message || "Failed to start passkey enrollment.",
        );
      }

      const credential = await createPasskeyCredential(
        optionsPayload.data.options,
      );
      await verifyPasskeyEnrollment.mutateAsync({
        body: {
          challenge_token: optionsPayload.data.challenge_token,
          credential,
        },
      });

      setSecurityFeedback({
        tone: "success",
        message: "Passkey added successfully.",
      });
    } catch (error) {
      setSecurityFeedback({
        tone: "error",
        message: getErrorMessage(error, "Failed to add passkey."),
      });
    }
  };

  const handleDeletePasskey = async (passkeyID: number) => {
    setSecurityFeedback(null);

    try {
      await deletePasskeyMutation.mutateAsync({
        params: {
          path: {
            id: passkeyID,
          },
        },
      });

      setSecurityFeedback({
        tone: "success",
        message: "Passkey removed successfully.",
      });
    } catch (error) {
      setSecurityFeedback({
        tone: "error",
        message: getErrorMessage(error, "Failed to remove passkey."),
      });
    }
  };

  const handleChangePassword = () => {
    navigate("/change-password", {
      state: {
        from: {
          pathname: "/settings",
          search: "?tab=account",
        },
      },
    });
  };

  const openPermissionsModal = () => {
    const modal = document.getElementById(
      "permissions_modal",
    ) as HTMLDialogElement | null;
    modal?.showModal();
  };

  /* ================================================================ */
  /*  Render                                                           */
  /* ================================================================ */

  return (
    <div className="flex flex-col gap-6">
      {/* ── Page heading ──────────────────────────────────────────── */}
      <div className="flex items-center gap-2.5">
        <UserCircleIcon className="size-6 text-primary" />
        <h2 className="text-2xl font-bold">
          {t("settings.account.title", { defaultValue: "Account" })}
        </h2>
      </div>

      {/* ── Profile: Avatar sidebar + Info form ──────────────────── */}
      <div className="grid gap-6 lg:grid-cols-[minmax(0,260px)_minmax(0,1fr)]">
        {/* Avatar card */}
        <div className="rounded-3xl border border-base-300 bg-base-100 p-6 shadow-sm">
          <div className="flex flex-col items-center gap-4 text-center">
            {/* Avatar */}
            <UserAvatar src={user.avatar_url} name={resolvedName} />

            {/* Name + username */}
            <div className="space-y-0.5">
              <div className="text-lg font-semibold text-base-content">
                {resolvedName}
              </div>
              <div className="text-sm text-base-content/60">
                @{user.username}
              </div>
            </div>

            {/* Role badge + View permissions */}
            <div className="flex flex-col items-center gap-2">
              <span className="badge badge-primary badge-outline font-medium">
                {(user.role ?? "user").toUpperCase()}
              </span>
              {(user.permissions ?? []).length > 0 && (
                <button
                  type="button"
                  className="btn btn-ghost btn-xs text-base-content/60"
                  onClick={openPermissionsModal}
                >
                  {t("settings.account.viewPermissions", {
                    defaultValue: "View permissions",
                  })}
                </button>
              )}
            </div>
          </div>
        </div>

        {/* Profile information form */}
        <div className="rounded-3xl border border-base-300 bg-base-100 p-6 shadow-sm">
          <h3 className="text-lg font-semibold text-base-content">
            {t("settings.account.profileInfo", {
              defaultValue: "Profile Information",
            })}
          </h3>
          <p className="mt-1 text-sm text-base-content/60">
            {t("settings.account.profileInfoHint", {
              defaultValue:
                "Update your display name and avatar. Username and role are managed by administrators.",
            })}
          </p>

          <form className="mt-5 space-y-5" onSubmit={handleProfileSubmit}>
            {profileFeedback && (
              <div
                className={`alert ${profileFeedback.tone === "success" ? "alert-success" : "alert-error"}`}
              >
                <span>{profileFeedback.message}</span>
              </div>
            )}

            {/* Display Name */}
            <fieldset className="fieldset">
              <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                {t("settings.account.displayName", {
                  defaultValue: "Display name",
                })}
              </legend>
              <input
                className="input input-bordered w-full bg-base-100"
                value={displayName}
                onChange={(event) => setDisplayName(event.target.value)}
                maxLength={DISPLAY_NAME_MAX_LENGTH}
                placeholder={user.username ?? "Display name"}
              />
              <p className="mt-1 text-xs text-base-content/60">
                {t("settings.account.displayNameHint", {
                  defaultValue: DISPLAY_NAME_HINT,
                })}
              </p>
            </fieldset>

            {/* Avatar URL */}
            <fieldset className="fieldset">
              <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                {t("settings.account.avatarUrl", {
                  defaultValue: "Avatar URL",
                })}
              </legend>
              <input
                className="input input-bordered w-full bg-base-100"
                value={avatarURL}
                onChange={(event) => setAvatarURL(event.target.value)}
                placeholder="https://example.com/avatar.jpg"
              />
              <p className="mt-1 text-xs text-base-content/60">
                {t("settings.account.avatarHint", {
                  defaultValue:
                    "Use an image URL. Leave empty to fall back to initials.",
                })}
              </p>
            </fieldset>

            {/* Read-only fields: Username + Role */}
            <div className="grid gap-4 sm:grid-cols-2">
              <fieldset className="fieldset">
                <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                  {t("settings.account.username", {
                    defaultValue: "Username",
                  })}
                </legend>
                <input
                  className="input input-bordered w-full bg-base-200/50"
                  value={user.username ?? ""}
                  disabled
                />
              </fieldset>

              <fieldset className="fieldset">
                <legend className="fieldset-legend text-sm font-medium text-base-content/80">
                  {t("settings.account.role", {
                    defaultValue: "Role",
                  })}
                </legend>
                <input
                  className="input input-bordered w-full bg-base-200/50"
                  value={(user.role ?? "user").toUpperCase()}
                  disabled
                />
              </fieldset>
            </div>

            {/* Submit */}
            <div className="flex justify-end pt-1">
              <button
                type="submit"
                className={`btn btn-primary ${updateProfileMutation.isPending ? "loading" : ""}`}
                disabled={!isDirty || updateProfileMutation.isPending}
              >
                {t("settings.account.save", {
                  defaultValue: "Save profile",
                })}
              </button>
            </div>
          </form>
        </div>
      </div>

      {/* ── Security section ─────────────────────────────────────── */}
      <div className="space-y-2">
        <div className="flex items-center gap-2.5">
          <ShieldCheckIcon className="size-6 text-primary" />
          <h2 className="text-2xl font-bold">
            {t("settings.account.securityTitle", {
              defaultValue: "Security",
            })}
          </h2>
        </div>
        <p className="text-sm text-base-content/70">
          {t("settings.account.securityDescription", {
            defaultValue:
              "Manage your password, authentication methods, and sign-in factors.",
          })}
        </p>
      </div>

      {/* Security link cards — same hierarchy level */}
      <div className="grid gap-4 xl:grid-cols-3">
        {/* Change Password */}
        <SecurityLinkCard
          icon={<KeyRound className="size-5" />}
          title={t("settings.account.passwordTitle", {
            defaultValue: "Password",
          })}
          description={t("settings.account.passwordDescription", {
            defaultValue:
              "Change your account password. You will be signed out afterwards.",
          })}
          onClick={handleChangePassword}
        />

        {/* Authenticator App */}
        <SecurityLinkCard
          icon={<ShieldCheckIcon className="size-5" />}
          title={t("settings.account.mfa.authenticatorAppTitle", {
            defaultValue: "Authenticator App",
          })}
          description={
            mfaStatus?.totp_enabled
              ? `${mfaStatus.recovery_codes_remaining ?? 0} ${t("settings.account.mfa.recoveryRemaining", { defaultValue: "recovery codes remaining" })}`
              : t("settings.account.mfa.notConfigured", {
                  defaultValue:
                    "Not configured yet. Secure your account with TOTP.",
                })
          }
          badge={
            mfaStatus?.totp_enabled
              ? t("settings.account.mfa.enabledBadge", {
                  defaultValue: "Enabled",
                })
              : t("settings.account.mfa.disabledBadge", {
                  defaultValue: "Disabled",
                })
          }
          badgeClass={
            mfaStatus?.totp_enabled
              ? "badge-success badge-outline"
              : "badge-ghost"
          }
          onClick={handleMFAToggle}
        />

        {/* Passkeys */}
        <SecurityLinkCard
          icon={<Fingerprint className="size-5" />}
          title={t("settings.account.mfa.passkeyTitle", {
            defaultValue: "Passkeys",
          })}
          description={
            passkeySupport.supported
              ? t("settings.account.mfa.passkeyDescription", {
                  defaultValue:
                    "Use your device's native passkey flow for sign-in.",
                })
              : passkeySupport.reasonKey
                ? t(passkeySupport.reasonKey)
                : ""
          }
          badge={
            passkeys.length > 0
              ? `${passkeys.length} ${t("settings.account.mfa.enrolled", { defaultValue: "enrolled" })}`
              : t("settings.account.mfa.notEnrolled", {
                  defaultValue: "Not enrolled",
                })
          }
          badgeClass={
            passkeys.length > 0 ? "badge-success badge-outline" : "badge-ghost"
          }
          onClick={openPasskeysModal}
        />
      </div>

      {/* Permissions modal */}
      <PermissionsModal
        permissions={user.permissions ?? []}
        role={user.role ?? "user"}
        t={t}
      />

      {/* Passkeys modal */}
      <PasskeysModal
        passkeys={passkeys}
        passkeysLoading={passkeysQuery.isLoading}
        passkeySupport={passkeySupport}
        passkeyBusy={passkeyBusy}
        deleteIsPending={deletePasskeyMutation.isPending}
        onAdd={() => void handleAddPasskey()}
        onDelete={(id) => void handleDeletePasskey(id)}
        feedback={securityFeedback}
        t={t}
      />
    </div>
  );
}
