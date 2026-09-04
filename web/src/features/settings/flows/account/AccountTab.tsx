import { useEffect, useMemo, useRef, useState } from "react";
import {
  Check,
  Fingerprint,
  KeyRound,
  MoveLeft,
  Plus,
  ShieldCheckIcon,
  Trash2,
  X,
} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import { localizeProblem } from "@/lib/http-commons/problem";
import UserAvatar from "@/components/ui/UserAvatar";
import {
  DISPLAY_NAME_HINT,
  DISPLAY_NAME_MAX_LENGTH,
  createPasskeyCredential,
  getPasskeySupport,
  getPasskeySupportMessage,
  useAuth,
  useBeginPasskeyEnrollment,
  useDeletePasskey,
  useMFAStatus,
  usePasskeys,
  useVerifyPasskeyEnrollment,
} from "@/features/auth";
import { useUpdateMyProfile } from "@/features/users";
import PhotoPicker from "@/features/assets/picker";
import type { components } from "@/lib/http-commons/schema";
import { SettingsGroup, SettingsRow, SettingsBlock } from "../../components/SettingsGroup";
import { SettingsSaveBar } from "../../components/SettingsSaveBar";

type Schemas = components["schemas"];
type FeedbackState = { tone: "success" | "error"; message: string } | null;
type UserDTO = Schemas["dto.UserDTO"];

const PERMISSION_KEYS = [
  "manage_users",
  "manage_settings",
  "view_all_assets",
  "manage_all_assets",
  "view_own_assets",
  "manage_own_assets",
  "manage_own_profile",
] as const;

function getPermissionLabel(key: string, t: ReturnType<typeof useI18n>["t"]): string {
  const labels: Record<string, string> = {
    manage_users: t("settings.account.permissions.manage_users.label", {
      defaultValue: "Manage Users",
    }),
    manage_settings: t("settings.account.permissions.manage_settings.label", {
      defaultValue: "Manage Settings",
    }),
    view_all_assets: t("settings.account.permissions.view_all_assets.label", {
      defaultValue: "View All Assets",
    }),
    manage_all_assets: t("settings.account.permissions.manage_all_assets.label", {
      defaultValue: "Manage All Assets",
    }),
    view_own_assets: t("settings.account.permissions.view_own_assets.label", {
      defaultValue: "View Own Assets",
    }),
    manage_own_assets: t("settings.account.permissions.manage_own_assets.label", {
      defaultValue: "Manage Own Assets",
    }),
    manage_own_profile: t("settings.account.permissions.manage_own_profile.label", {
      defaultValue: "Manage Own Profile",
    }),
  };
  return labels[key] ?? key.replace(/_/g, " ").replace(/\b\w/g, (c: string) => c.toUpperCase());
}

function getPermissionDescription(key: string, t: ReturnType<typeof useI18n>["t"]): string {
  const descriptions: Record<string, string> = {
    manage_users: t("settings.account.permissions.manage_users.description", {
      defaultValue: "Create, edit, and deactivate user accounts.",
    }),
    manage_settings: t("settings.account.permissions.manage_settings.description", {
      defaultValue: "Modify global application settings.",
    }),
    view_all_assets: t("settings.account.permissions.view_all_assets.description", {
      defaultValue: "Browse all photos and media across Repositories.",
    }),
    manage_all_assets: t("settings.account.permissions.manage_all_assets.description", {
      defaultValue: "Edit, delete, and modify metadata for any asset.",
    }),
    view_own_assets: t("settings.account.permissions.view_own_assets.description", {
      defaultValue: "Browse your own uploaded photos and media.",
    }),
    manage_own_assets: t("settings.account.permissions.manage_own_assets.description", {
      defaultValue: "Edit, delete, and modify metadata on your own assets.",
    }),
    manage_own_profile: t("settings.account.permissions.manage_own_profile.description", {
      defaultValue: "Update your display name, avatar, and security settings.",
    }),
  };
  return descriptions[key] ?? "";
}

function isValidUserDTO(value: unknown): value is UserDTO {
  if (!value || typeof value !== "object") return false;
  const candidate = value as UserDTO;
  return typeof candidate.user_id === "number" && Boolean(candidate.username);
}

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
  const extraPermissions = useMemo(
    () => permissions.filter((p) => !(PERMISSION_KEYS as readonly string[]).includes(p)),
    [permissions],
  );

  return (
    <dialog id="permissions_modal" className="modal modal-bottom sm:modal-middle">
      <div className="modal-box max-w-lg">
        <form method="dialog">
          <button className="btn btn-sm btn-circle btn-ghost absolute right-3 top-3">
            <X className="size-4" />
          </button>
        </form>
        <h3 className="text-lg font-bold">
          {t("settings.account.permissionsTitle", { defaultValue: "Account Permissions" })}
        </h3>
        <p className="mt-1 text-sm text-base-content/70">
          {t("settings.account.permissionsSubtitle", {
            defaultValue: "Permissions assigned to your account based on your role.",
          })}
        </p>
        <div className="mt-1 mb-4">
          <span className="badge badge-primary badge-sm">{role.toUpperCase()}</span>
        </div>
        <div className="divide-y divide-base-200">
          {PERMISSION_KEYS.map((key) => {
            const granted = permissionSet.has(key);
            return (
              <div key={key} className="flex items-center gap-3 py-3">
                <div
                  className={`flex size-7 shrink-0 items-center justify-center rounded-full ${
                    granted ? "bg-success/15 text-success" : "bg-base-200 text-base-content/30"
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
                    {getPermissionLabel(key, t)}
                  </div>
                  <div className="text-xs text-base-content/60">
                    {getPermissionDescription(key, t)}
                  </div>
                </div>
              </div>
            );
          })}
          {extraPermissions.map((perm) => (
            <div key={perm} className="flex items-center gap-3 py-3">
              <div className="flex size-7 shrink-0 items-center justify-center rounded-full bg-success/15 text-success">
                <Check className="size-4" strokeWidth={2.5} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="text-sm font-medium text-base-content">
                  {perm.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase())}
                </div>
                <div className="text-xs text-base-content/60">
                  {t("settings.account.permissions.customPermission", {
                    defaultValue: "Custom permission",
                  })}
                </div>
              </div>
            </div>
          ))}
        </div>
        <div className="modal-action">
          <form method="dialog">
            <button className="btn btn-sm">{t("common.close", { defaultValue: "Close" })}</button>
          </form>
        </div>
      </div>
      <form method="dialog" className="modal-backdrop">
        <button>close</button>
      </form>
    </dialog>
  );
}

function PasskeysModal({
  passkeys,
  passkeysLoading,
  passkeySupport,
  passkeyBusy,
  deleteIsPending,
  totpEnabled,
  onAdd,
  onDelete,
  feedback,
  securityPassword,
  setSecurityPassword,
  securityCode,
  setSecurityCode,
  securityMethod,
  setSecurityMethod,
  securityVerifyPending,
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
  totpEnabled: boolean;
  onAdd: () => void;
  onDelete: (id: number) => void;
  feedback: FeedbackState;
  securityPassword: string;
  setSecurityPassword: (value: string) => void;
  securityCode: string;
  setSecurityCode: (value: string) => void;
  securityMethod: "totp" | "recovery_code";
  setSecurityMethod: (value: "totp" | "recovery_code") => void;
  securityVerifyPending: boolean;
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
        <div className="flex items-center gap-3">
          <div className="flex size-10 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary">
            <Fingerprint className="size-5" />
          </div>
          <div>
            <h3 className="text-lg font-bold">
              {t("settings.account.mfa.enrolledPasskeys", { defaultValue: "Passkeys" })}
            </h3>
            <p className="text-sm text-base-content/60">
              {t("settings.account.mfa.enrolledPasskeysHint", {
                defaultValue: "Manage the passkey credentials registered to this account.",
              })}
            </p>
          </div>
        </div>
        {feedback && (
          <div
            className={`alert mt-4 ${feedback.tone === "success" ? "alert-success" : "alert-error"}`}
          >
            <span>{feedback.message}</span>
          </div>
        )}
        <div className="mt-4 rounded-2xl border border-base-300 bg-base-200/30 p-4">
          <p className="text-sm font-semibold text-base-content">
            {t("settings.account.mfa.passkeySecurityTitle", {
              defaultValue: "Confirm account security",
            })}
          </p>
          <p className="mt-1 text-xs text-base-content/60">
            {t("settings.account.mfa.passkeySecurityHint", {
              defaultValue:
                "Enter your password and a current authenticator code before changing passkeys.",
            })}
          </p>
          <div className="mt-3 grid gap-2 sm:grid-cols-3">
            <input
              type="password"
              className="input input-bordered input-sm w-full"
              placeholder={t("auth.login.password", { defaultValue: "Password" })}
              autoComplete="current-password"
              value={securityPassword}
              onChange={(event) => setSecurityPassword(event.target.value)}
            />
            <select
              className="select select-bordered select-sm w-full"
              value={securityMethod}
              onChange={(event) =>
                setSecurityMethod(event.target.value as "totp" | "recovery_code")
              }
            >
              <option value="totp">
                {t("auth.mfa.totpCode", { defaultValue: "Authenticator code" })}
              </option>
              <option value="recovery_code">
                {t("auth.mfa.recoveryCode", { defaultValue: "Recovery code" })}
              </option>
            </select>
            <input
              inputMode="numeric"
              className="input input-bordered input-sm w-full"
              placeholder={t("settings.account.mfa.totpCode", {
                defaultValue: "Authenticator code",
              })}
              value={securityCode}
              onChange={(event) => setSecurityCode(event.target.value)}
            />
          </div>
        </div>
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
                    : getPasskeySupportMessage(t, passkeySupport.reasonKey)}
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
                  <div className="text-sm font-semibold text-base-content">{passkey.label}</div>
                  <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-0.5 text-xs text-base-content/55">
                    <span>
                      {t("settings.account.mfa.passkeyCreated", { defaultValue: "Created" })}{" "}
                      {new Date(passkey.created_at ?? "").toLocaleDateString()}
                    </span>
                    {passkey.last_used_at && (
                      <span>
                        {t("settings.account.mfa.passkeyLastUsed", { defaultValue: "Last used" })}{" "}
                        {new Date(passkey.last_used_at).toLocaleDateString()}
                      </span>
                    )}
                  </div>
                </div>
                <button
                  type="button"
                  className="btn btn-ghost btn-sm btn-square text-error"
                  disabled={deleteIsPending || securityVerifyPending || !passkey.passkey_id}
                  onClick={() => onDelete(passkey.passkey_id ?? 0)}
                  title={t("common.remove", { defaultValue: "Remove" })}
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            ))
          )}
        </div>
        {passkeySupport.supported && !totpEnabled && (
          <p className="mt-2 text-xs text-base-content/55">
            {t("settings.account.mfa.passkeyRequiresTotp", {
              defaultValue:
                "Enable an authenticator app (TOTP) first — a passkey can only be added on top of it.",
            })}
          </p>
        )}
        <div className="modal-action">
          <button
            type="button"
            className={`btn btn-primary btn-sm gap-1.5 ${passkeyBusy ? "loading" : ""}`}
            disabled={
              !passkeySupport.supported ||
              !totpEnabled ||
              passkeyBusy ||
              securityVerifyPending ||
              !securityPassword ||
              !securityCode
            }
            onClick={onAdd}
          >
            <Plus className="size-4" />
            {t("settings.account.mfa.addPasskey", { defaultValue: "Add Passkey" })}
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

export default function AccountTab() {
  const { t } = useI18n();
  const { user, dispatch, completeAuth } = useAuth();
  const navigate = useNavigate();
  const mfaStatusQuery = useMFAStatus();
  const passkeysQuery = usePasskeys();
  const updateProfileMutation = useUpdateMyProfile();
  const beginPasskeyEnrollment = useBeginPasskeyEnrollment();
  const verifyPasskeyEnrollment = useVerifyPasskeyEnrollment();
  const deletePasskeyMutation = useDeletePasskey();
  const securityVerifyMutation = $api.useMutation("post", "/api/v1/auth/security/verify");

  const [displayName, setDisplayName] = useState(() => user?.display_name ?? "");
  const [avatarAssetId, setAvatarAssetId] = useState(() => user?.avatar_asset_id ?? "");
  const [profileTouched, setProfileTouched] = useState(false);
  const [isChoosingAvatar, setIsChoosingAvatar] = useState(false);
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileJustSaved, setProfileJustSaved] = useState(false);
  const [securityFeedback, setSecurityFeedback] = useState<FeedbackState>(null);
  const [securityPassword, setSecurityPassword] = useState("");
  const [securityCode, setSecurityCode] = useState("");
  const [securityMethod, setSecurityMethod] = useState<"totp" | "recovery_code">("totp");
  const profileSavedTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const passkeySupport = useMemo(() => getPasskeySupport(), []);

  const openPasskeysModal = () => {
    const modal = document.getElementById("passkeys_modal") as HTMLDialogElement | null;
    modal?.showModal();
  };
  const openPermissionsModal = () => {
    const modal = document.getElementById("permissions_modal") as HTMLDialogElement | null;
    modal?.showModal();
  };

  useEffect(() => {
    if (profileTouched) return;
    setDisplayName(user?.display_name ?? "");
    setAvatarAssetId(user?.avatar_asset_id ?? "");
  }, [profileTouched, user?.display_name, user?.avatar_asset_id]);

  useEffect(
    () => () => {
      if (profileSavedTimer.current) clearTimeout(profileSavedTimer.current);
    },
    [],
  );

  if (!user) {
    return (
      <div className="w-full rounded-2xl bg-base-200/50 px-4 py-6 text-sm text-base-content/60">
        {t("common.loading", { defaultValue: "Loading..." })}
      </div>
    );
  }

  const resolvedName = user.display_name || user.username || "User";
  const mfaStatus = mfaStatusQuery.data;
  const effectiveAvatarAssetId = avatarAssetId || undefined;
  const isDirty =
    profileTouched &&
    (displayName !== (user.display_name ?? "") || avatarAssetId !== (user.avatar_asset_id ?? ""));
  const passkeys = passkeysQuery.passkeys;
  const mfaIsLoading = mfaStatusQuery.isLoading || mfaStatusQuery.isFetching;
  const passkeysAreLoading = passkeysQuery.isLoading || passkeysQuery.isFetching;
  const mfaLoadError = mfaStatusQuery.isError;
  const passkeysLoadError = passkeysQuery.isError;
  const passkeyBusy =
    beginPasskeyEnrollment.isPending ||
    verifyPasskeyEnrollment.isPending ||
    deletePasskeyMutation.isPending ||
    securityVerifyMutation.isPending;

  const handleMFAToggle = () => {
    if (mfaIsLoading || mfaLoadError || !mfaStatus) return;
    const search = mfaStatus?.totp_enabled ? "?action=disable" : "?mfa=setup";
    void navigate(`/mfa${search}`, {
      state: { from: { pathname: "/settings", search: "?tab=account" } },
    });
  };

  const resetProfileDraft = () => {
    setProfileTouched(false);
    setDisplayName(user.display_name ?? "");
    setAvatarAssetId(user.avatar_asset_id ?? "");
    setProfileError(null);
    setProfileJustSaved(false);
  };

  const handleProfileSave = async () => {
    if (!isDirty || updateProfileMutation.isPending) return;
    setProfileError(null);
    setProfileJustSaved(false);
    try {
      const response = await updateProfileMutation.mutateAsync({
        body: { display_name: displayName, avatar_asset_id: avatarAssetId },
      });
      if (!isValidUserDTO(response)) {
        setProfileError(
          t("settings.account.invalidProfileResponse", {
            defaultValue: "Profile saved, but the server returned an invalid user payload.",
          }),
        );
        return;
      }
      dispatch({ type: "SET_USER", payload: response });
      setProfileTouched(false);
      setProfileJustSaved(true);
      if (profileSavedTimer.current) clearTimeout(profileSavedTimer.current);
      profileSavedTimer.current = setTimeout(() => setProfileJustSaved(false), 2500);
    } catch (error) {
      setProfileError(
        localizeProblem(
          error,
          t,
          t("settings.account.saveError", { defaultValue: "Failed to update profile." }),
        ),
      );
    }
  };

  const handleAddPasskey = async () => {
    setSecurityFeedback(null);
    try {
      const security = await securityVerifyMutation.mutateAsync({
        body: {
          current_password: securityPassword,
          code: securityCode,
          method: securityMethod,
          purpose: "passkey_mutation",
        },
      });
      if (!security?.security_token) throw new Error("Security verification failed.");
      const optionsResponse = await beginPasskeyEnrollment.mutateAsync({});
      if (!optionsResponse.challenge_token) {
        throw new Error("Failed to start passkey enrollment.");
      }
      const credential = await createPasskeyCredential(optionsResponse.options);
      const response = await verifyPasskeyEnrollment.mutateAsync({
        body: {
          challenge_token: optionsResponse.challenge_token,
          credential,
          security_token: security.security_token,
        },
      });
      if (!response.session) throw new Error("Session replacement was not returned.");
      await completeAuth(response.session);
      setSecurityPassword("");
      setSecurityCode("");
      setSecurityMethod("totp");
      setSecurityFeedback({
        tone: "success",
        message: t("settings.account.passkeys.added", "Passkey added successfully."),
      });
    } catch (error) {
      setSecurityFeedback({
        tone: "error",
        message: localizeProblem(
          error,
          t,
          t("settings.account.passkeys.addFailed", "Failed to add passkey."),
        ),
      });
    }
  };

  const handleDeletePasskey = async (passkeyID: number) => {
    setSecurityFeedback(null);
    if (!Number.isFinite(passkeyID) || passkeyID <= 0) {
      setSecurityFeedback({
        tone: "error",
        message: t("settings.account.passkeys.invalidID", "Invalid passkey ID."),
      });
      return;
    }
    try {
      const security = await securityVerifyMutation.mutateAsync({
        body: {
          current_password: securityPassword,
          code: securityCode,
          method: securityMethod,
          purpose: "passkey_mutation",
        },
      });
      if (!security?.security_token) throw new Error("Security verification failed.");
      const response = await deletePasskeyMutation.mutateAsync({
        params: { path: { id: passkeyID } },
        body: { security_token: security.security_token },
      });
      if (!response.session) throw new Error("Session replacement was not returned.");
      await completeAuth(response.session);
      setSecurityPassword("");
      setSecurityCode("");
      setSecurityMethod("totp");
      setSecurityFeedback({
        tone: "success",
        message: t("settings.account.passkeys.removed", "Passkey removed successfully."),
      });
    } catch (error) {
      setSecurityFeedback({
        tone: "error",
        message: localizeProblem(
          error,
          t,
          t("settings.account.passkeys.removeFailed", "Failed to remove passkey."),
        ),
      });
    }
  };

  const handleChangePassword = () => {
    void navigate("/change-password", {
      state: { from: { pathname: "/settings", search: "?tab=account" } },
    });
  };

  const handleAvatarSelect = (assetId: string) => {
    setProfileTouched(true);
    setAvatarAssetId(assetId);
    setIsChoosingAvatar(false);
  };

  const mfaBadge = mfaIsLoading
    ? { label: t("common.loading", { defaultValue: "Loading..." }), cls: "badge-ghost" }
    : mfaLoadError
      ? { label: t("common.error", { defaultValue: "Error" }), cls: "badge-error badge-outline" }
      : mfaStatus?.totp_enabled
        ? {
            label: t("settings.account.mfa.enabledBadge", { defaultValue: "Enabled" }),
            cls: "badge-success badge-outline",
          }
        : {
            label: t("settings.account.mfa.disabledBadge", { defaultValue: "Disabled" }),
            cls: "badge-ghost",
          };

  const passkeysBadge = passkeysAreLoading
    ? { label: t("common.loading", { defaultValue: "Loading..." }), cls: "badge-ghost" }
    : passkeysLoadError
      ? { label: t("common.error", { defaultValue: "Error" }), cls: "badge-error badge-outline" }
      : passkeys.length > 0
        ? {
            label: `${passkeys.length} ${t("settings.account.mfa.enrolled", { defaultValue: "enrolled" })}`,
            cls: "badge-success badge-outline",
          }
        : {
            label: t("settings.account.mfa.notEnrolled", { defaultValue: "Not enrolled" }),
            cls: "badge-ghost",
          };

  return (
    <div className="w-full space-y-8 lg:space-y-10">
      <SettingsGroup
        title={t("settings.account.profileInfo", { defaultValue: "Profile Information" })}
        description={t("settings.account.profileInfoHint", {
          defaultValue:
            "Update your display name and avatar. Username and role are managed by administrators.",
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
              <div className="text-sm text-base-content/60">@{user.username}</div>
              <div className="mt-1.5 flex flex-wrap items-center gap-2">
                <span className="badge badge-primary badge-outline badge-sm font-medium">
                  {(user.role ?? "user").toUpperCase()}
                </span>
                {(user.permissions ?? []).length > 0 && (
                  <button
                    type="button"
                    className="btn btn-ghost btn-xs text-base-content/60"
                    onClick={openPermissionsModal}
                  >
                    {t("settings.account.viewPermissions", { defaultValue: "View permissions" })}
                  </button>
                )}
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                className="btn btn-outline btn-sm"
                onClick={() => setIsChoosingAvatar(true)}
              >
                {effectiveAvatarAssetId
                  ? t("settings.account.changeAvatar", { defaultValue: "Change photo" })
                  : t("settings.account.chooseAvatar", { defaultValue: "Choose photo" })}
              </button>
              {effectiveAvatarAssetId && (
                <button
                  type="button"
                  className="btn btn-ghost btn-sm"
                  onClick={() => {
                    setProfileTouched(true);
                    setAvatarAssetId("");
                  }}
                >
                  {t("settings.account.removeAvatar", { defaultValue: "Remove photo" })}
                </button>
              )}
            </div>
          </div>
        </SettingsBlock>

        <SettingsBlock>
          <label htmlFor="account-display-name" className="text-sm font-medium">
            {t("settings.account.displayName", { defaultValue: "Display name" })}
          </label>
          <input
            id="account-display-name"
            className="input input-bordered input-sm mt-2 w-full"
            value={displayName}
            onChange={(event) => {
              setProfileTouched(true);
              setDisplayName(event.target.value);
            }}
            maxLength={DISPLAY_NAME_MAX_LENGTH}
            placeholder={user.username ?? "Display name"}
          />
          <p className="mt-1.5 text-xs text-base-content/55">
            {t("settings.account.displayNameHint", { defaultValue: DISPLAY_NAME_HINT })}
          </p>
        </SettingsBlock>

        <SettingsRow
          label={t("settings.account.username", { defaultValue: "Username" })}
          value={<span className="font-mono">{user.username ?? "—"}</span>}
        />
        <SettingsRow
          label={t("settings.account.role", { defaultValue: "Role" })}
          value={(user.role ?? "user").toUpperCase()}
        />
      </SettingsGroup>

      <SettingsGroup
        title={t("settings.account.securityTitle", { defaultValue: "Security" })}
        description={t("settings.account.securityDescription", {
          defaultValue: "Manage your password, authentication methods, and sign-in factors.",
        })}
      >
        {securityFeedback && (
          <SettingsBlock>
            <div
              className={`rounded-lg px-3 py-2 text-sm ${
                securityFeedback.tone === "success"
                  ? "bg-success/10 text-success"
                  : "bg-error/10 text-error"
              }`}
            >
              {securityFeedback.message}
            </div>
          </SettingsBlock>
        )}
        <SettingsRow
          icon={<KeyRound className="size-4" />}
          iconColor="bg-info text-info-content"
          label={t("settings.account.passwordTitle", { defaultValue: "Password" })}
          description={t("settings.account.passwordDescription", {
            defaultValue: "Change your account password. You will be signed out afterwards.",
          })}
          chevron
          onClick={handleChangePassword}
        />
        <SettingsRow
          icon={<ShieldCheckIcon className="size-4" />}
          iconColor="bg-success text-success-content"
          label={t("settings.account.mfa.authenticatorAppTitle", {
            defaultValue: "Authenticator App",
          })}
          description={
            mfaIsLoading
              ? t("common.loading", { defaultValue: "Loading..." })
              : mfaLoadError
                ? t("settings.account.mfa.statusLoadError", {
                    defaultValue: "MFA status is temporarily unavailable.",
                  })
                : mfaStatus?.totp_enabled
                  ? `${mfaStatus.recovery_codes_remaining ?? 0} ${t("settings.account.mfa.recoveryRemaining", { defaultValue: "recovery codes remaining" })}`
                  : t("settings.account.mfa.notConfigured", {
                      defaultValue: "Not configured yet. Secure your account with TOTP.",
                    })
          }
          value={<span className={`badge badge-sm ${mfaBadge.cls}`}>{mfaBadge.label}</span>}
          chevron
          disabled={mfaIsLoading || mfaLoadError || !mfaStatus}
          onClick={handleMFAToggle}
        />
        <SettingsRow
          icon={<Fingerprint className="size-4" />}
          iconColor="bg-primary text-primary-content"
          label={t("settings.account.mfa.passkeyTitle", { defaultValue: "Passkeys" })}
          description={
            passkeysAreLoading
              ? t("common.loading", { defaultValue: "Loading..." })
              : passkeysLoadError
                ? t("settings.account.mfa.passkeyLoadError", {
                    defaultValue: "Passkeys are temporarily unavailable.",
                  })
                : passkeySupport.supported
                  ? t("settings.account.mfa.passkeyDescription", {
                      defaultValue: "Use your device's native passkey flow for sign-in.",
                    })
                  : getPasskeySupportMessage(t, passkeySupport.reasonKey)
          }
          value={
            <span className={`badge badge-sm ${passkeysBadge.cls}`}>{passkeysBadge.label}</span>
          }
          chevron
          disabled={passkeysAreLoading || passkeysLoadError}
          onClick={openPasskeysModal}
        />
      </SettingsGroup>

      <SettingsSaveBar
        isDirty={isDirty}
        isSaving={updateProfileMutation.isPending}
        justSaved={profileJustSaved}
        localizedError={profileError}
        canSave={isDirty && !updateProfileMutation.isPending}
        onSave={() => void handleProfileSave()}
        onReset={resetProfileDraft}
      />

      {isChoosingAvatar && (
        <div className="fixed inset-0 z-modal flex flex-col bg-base-100 animate-in slide-in-from-bottom duration-300">
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
              scopeId="photo-picker:account-avatar"
              onSelect={handleAvatarSelect}
              title={t("settings.account.avatarPhoto", { defaultValue: "Avatar photo" })}
            />
          </div>
        </div>
      )}

      <PermissionsModal permissions={user.permissions ?? []} role={user.role ?? "user"} t={t} />
      <PasskeysModal
        passkeys={passkeys}
        passkeysLoading={passkeysQuery.isLoading}
        passkeySupport={passkeySupport}
        passkeyBusy={passkeyBusy}
        deleteIsPending={deletePasskeyMutation.isPending}
        totpEnabled={mfaStatus?.totp_enabled ?? false}
        onAdd={() => void handleAddPasskey()}
        onDelete={(id) => void handleDeletePasskey(id)}
        feedback={securityFeedback}
        securityPassword={securityPassword}
        setSecurityPassword={setSecurityPassword}
        securityCode={securityCode}
        setSecurityCode={setSecurityCode}
        securityMethod={securityMethod}
        setSecurityMethod={setSecurityMethod}
        securityVerifyPending={securityVerifyMutation.isPending}
        t={t}
      />
    </div>
  );
}
