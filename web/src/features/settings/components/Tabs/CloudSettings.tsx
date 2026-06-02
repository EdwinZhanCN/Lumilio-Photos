import { useMemo, useState, type FormEvent } from "react";
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  CloudIcon,
  KeyRoundIcon,
  LogOutIcon,
  PlusIcon,
  ShieldCheckIcon,
  XIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useCloudCredentials,
  useCreateICloudCredential,
  useDisableCloudCredential,
  useVerifyICloudCredential2FA,
  type CloudCredential,
} from "../../hooks/useCloudSync";

type ProviderChoice = "icloud";

const credentialStatusClass = (status?: string) => {
  switch (status) {
    case "connected":
      return "badge-success";
    case "pending_2fa":
      return "badge-warning";
    case "error":
      return "badge-error";
    default:
      return "badge-ghost";
  }
};

export default function CloudSettings() {
  const { t } = useI18n();
  const credentialsQuery = useCloudCredentials();
  const createCredential = useCreateICloudCredential();
  const verify2FA = useVerifyICloudCredential2FA();
  const disableCredential = useDisableCloudCredential();

  const [isAddOpen, setIsAddOpen] = useState(false);
  const [providerChoice, setProviderChoice] = useState<ProviderChoice | null>(null);
  const [displayName, setDisplayName] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [domain, setDomain] = useState<"com" | "cn">("com");
  const [pendingCredential, setPendingCredential] = useState<CloudCredential | null>(null);
  const [verificationCode, setVerificationCode] = useState("");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);

  const credentials = useMemo(
    () => credentialsQuery.data?.data?.credentials ?? [],
    [credentialsQuery.data],
  );
  const connectedCount = credentials.filter((item) => item.status === "connected").length;

  const resetForm = () => {
    setDisplayName("");
    setUsername("");
    setPassword("");
    setDomain("com");
    setVerificationCode("");
    setPendingCredential(null);
    setProviderChoice(null);
  };

  const closeModal = () => {
    if (createCredential.isPending || verify2FA.isPending) return;
    resetForm();
    setIsAddOpen(false);
  };

  const openModal = () => {
    setErrorMsg(null);
    setSuccessMsg(null);
    resetForm();
    setIsAddOpen(true);
  };

  const statusLabel = (status?: string) => {
    switch (status) {
      case "connected":
        return t("settings.cloud.status.connected");
      case "pending_2fa":
        return t("settings.cloud.status.pending2FA");
      case "error":
        return t("settings.cloud.status.error");
      case "disabled":
        return t("settings.cloud.status.disabled");
      default:
        return t("settings.cloud.status.unknown");
    }
  };

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setErrorMsg(null);
    setSuccessMsg(null);

    try {
      const result = await createCredential.mutateAsync({
        body: {
          display_name: displayName.trim() || undefined,
          username: username.trim(),
          password,
          domain,
        },
      });
      const credential = result.data?.credential;
      if (!credential) {
        throw new Error(t("settings.cloud.errors.emptyCredentialResponse"));
      }
      if (result.data?.needs_2fa) {
        setPendingCredential(credential);
        setSuccessMsg(t("settings.cloud.messages.verificationSent"));
      } else {
        resetForm();
        setIsAddOpen(false);
        setSuccessMsg(t("settings.cloud.messages.connected"));
      }
    } catch (error) {
      setErrorMsg(
        error instanceof Error ? error.message : t("settings.cloud.errors.createFailed"),
      );
    }
  };

  const handleVerify = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!pendingCredential) return;
    setErrorMsg(null);
    setSuccessMsg(null);

    try {
      await verify2FA.mutateAsync({
        params: {
          path: {
            id: pendingCredential.id,
          },
        },
        body: {
          code: verificationCode,
        },
      });
      resetForm();
      setIsAddOpen(false);
      setSuccessMsg(t("settings.cloud.messages.verified"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.verifyFailed"));
    }
  };

  const handleDisable = async (credential: CloudCredential) => {
    if (
      !confirm(
        t("settings.cloud.confirmDisable", {
          name: credential.display_name,
        }),
      )
    ) {
      return;
    }
    setErrorMsg(null);
    setSuccessMsg(null);

    try {
      await disableCredential.mutateAsync({
        params: {
          path: {
            id: credential.id,
          },
        },
      });
      setSuccessMsg(t("settings.cloud.messages.disabled"));
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : t("settings.cloud.errors.disableFailed"));
    }
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <div className="rounded-lg bg-primary/10 p-2 text-primary">
            <CloudIcon className="size-6" />
          </div>
          <h2 className="text-2xl font-bold">{t("settings.cloud.title")}</h2>
        </div>
        <p className="text-sm text-base-content/65">{t("settings.cloud.description")}</p>
      </header>

      {errorMsg && (
        <div className="alert alert-error rounded-lg">
          <AlertTriangleIcon className="size-5" />
          <span>{errorMsg}</span>
        </div>
      )}
      {successMsg && (
        <div className="alert alert-success rounded-lg">
          <CheckCircle2Icon className="size-5" />
          <span>{successMsg}</span>
        </div>
      )}

      <section className="space-y-3">
        <div className="flex items-center justify-between gap-3">
          <div>
            <h3 className="text-base font-semibold">{t("settings.cloud.savedTitle")}</h3>
            <p className="text-sm text-base-content/60">
              {t("settings.cloud.connectedAccounts", { count: connectedCount })}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {credentialsQuery.isLoading && <span className="loading loading-spinner loading-sm" />}
            <button
              type="button"
              className="btn btn-primary btn-sm btn-circle"
              onClick={openModal}
              aria-label={t("settings.cloud.addCredential")}
              title={t("settings.cloud.addCredential")}
            >
              <PlusIcon size={18} />
            </button>
          </div>
        </div>

        {credentials.length === 0 && !credentialsQuery.isLoading ? (
          <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
            {t("settings.cloud.empty")}
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {credentials.map((credential) => (
              <article
                key={credential.id}
                className="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm"
              >
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-start gap-3">
                    <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                      <KeyRoundIcon size={20} />
                    </div>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <h4 className="truncate text-sm font-semibold">
                          {credential.display_name}
                        </h4>
                        <span
                          className={`badge badge-sm ${credentialStatusClass(credential.status)}`}
                        >
                          {statusLabel(credential.status)}
                        </span>
                      </div>
                      <p className="mt-1 text-xs text-base-content/60">
                        {t("settings.cloud.credentialMeta", {
                          account: credential.masked_account,
                          domain: credential.domain,
                        })}
                      </p>
                    </div>
                  </div>
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm gap-2 text-error"
                    onClick={() => handleDisable(credential)}
                    disabled={disableCredential.isPending || credential.status === "disabled"}
                  >
                    <LogOutIcon size={15} />
                    {t("settings.cloud.disable")}
                  </button>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>

      <div className="rounded-lg border border-base-300 bg-base-200/40 p-4 text-sm text-base-content/70">
        {t("settings.cloud.securityNote")}
      </div>

      {isAddOpen && (
        <div className="modal modal-open">
          <div className="modal-box max-w-lg rounded-lg">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-semibold">
                  {pendingCredential
                    ? t("settings.cloud.verifyTitle")
                    : providerChoice
                      ? t("settings.cloud.icloudFormTitle")
                      : t("settings.cloud.providerTitle")}
                </h3>
                <p className="mt-1 text-sm text-base-content/65">
                  {pendingCredential
                    ? t("settings.cloud.verifyDescription", {
                        name: pendingCredential.display_name,
                      })
                    : providerChoice
                      ? t("settings.cloud.icloudFormDescription")
                      : t("settings.cloud.providerDescription")}
                </p>
              </div>
              <button
                type="button"
                className="btn btn-ghost btn-sm btn-circle"
                onClick={closeModal}
                disabled={createCredential.isPending || verify2FA.isPending}
                aria-label={t("common.close", { defaultValue: "Close" })}
              >
                <XIcon size={18} />
              </button>
            </div>

            {!providerChoice && !pendingCredential && (
              <div className="grid gap-3">
                <button
                  type="button"
                  className="flex w-full items-center gap-3 rounded-lg border border-base-300 bg-base-100 p-4 text-left transition hover:border-primary"
                  onClick={() => setProviderChoice("icloud")}
                >
                  <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
                    <CloudIcon size={20} />
                  </div>
                  <div>
                    <div className="font-semibold">
                      {t("settings.cloud.providers.icloud.title")}
                    </div>
                    <div className="text-sm text-base-content/60">
                      {t("settings.cloud.providers.icloud.description")}
                    </div>
                  </div>
                </button>
              </div>
            )}

            {providerChoice === "icloud" && !pendingCredential && (
              <form onSubmit={handleCreate} className="space-y-4">
                <label className="form-control w-full">
                  <span className="label-text pb-1 text-sm font-medium">
                    {t("settings.cloud.fields.label")}
                  </span>
                  <input
                    type="text"
                    className="input input-bordered w-full"
                    value={displayName}
                    onChange={(event) => setDisplayName(event.target.value)}
                    placeholder={t("settings.cloud.placeholders.label")}
                    disabled={createCredential.isPending}
                  />
                </label>

                <label className="form-control w-full">
                  <span className="label-text pb-1 text-sm font-medium">
                    {t("settings.cloud.fields.appleId")}
                  </span>
                  <input
                    type="email"
                    className="input input-bordered w-full"
                    value={username}
                    onChange={(event) => setUsername(event.target.value)}
                    placeholder={t("settings.cloud.placeholders.appleId")}
                    disabled={createCredential.isPending}
                    required
                  />
                </label>

                <label className="form-control w-full">
                  <span className="label-text pb-1 text-sm font-medium">
                    {t("settings.cloud.fields.password")}
                  </span>
                  <input
                    type="password"
                    className="input input-bordered w-full font-mono"
                    value={password}
                    onChange={(event) => setPassword(event.target.value)}
                    placeholder={t("settings.cloud.placeholders.password")}
                    disabled={createCredential.isPending}
                    required
                  />
                </label>

                <label className="form-control w-full">
                  <span className="label-text pb-1 text-sm font-medium">
                    {t("settings.cloud.fields.domain")}
                  </span>
                  <select
                    className="select select-bordered w-full"
                    value={domain}
                    onChange={(event) => setDomain(event.target.value as "com" | "cn")}
                    disabled={createCredential.isPending}
                  >
                    <option value="com">{t("settings.cloud.domain.global")}</option>
                    <option value="cn">{t("settings.cloud.domain.china")}</option>
                  </select>
                </label>

                <div className="modal-action">
                  <button
                    type="button"
                    className="btn btn-ghost"
                    onClick={() => setProviderChoice(null)}
                    disabled={createCredential.isPending}
                  >
                    {t("common.back", { defaultValue: "Back" })}
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary gap-2"
                    disabled={!username.trim() || !password || createCredential.isPending}
                  >
                    {createCredential.isPending ? (
                      <span className="loading loading-spinner loading-xs" />
                    ) : (
                      <ShieldCheckIcon size={16} />
                    )}
                    {t("settings.cloud.connectCredential")}
                  </button>
                </div>
              </form>
            )}

            {pendingCredential && (
              <form onSubmit={handleVerify} className="space-y-4">
                <input
                  type="text"
                  maxLength={6}
                  className="input input-bordered w-full text-center text-xl font-semibold tracking-widest"
                  value={verificationCode}
                  onChange={(event) => setVerificationCode(event.target.value.replace(/\D/g, ""))}
                  disabled={verify2FA.isPending}
                  autoFocus
                  required
                />
                <div className="modal-action">
                  <button
                    type="button"
                    className="btn btn-ghost"
                    disabled={verify2FA.isPending}
                    onClick={closeModal}
                  >
                    {t("common.cancel", { defaultValue: "Cancel" })}
                  </button>
                  <button
                    type="submit"
                    className="btn btn-primary"
                    disabled={verificationCode.length !== 6 || verify2FA.isPending}
                  >
                    {verify2FA.isPending && <span className="loading loading-spinner loading-xs" />}
                    {t("settings.cloud.verify")}
                  </button>
                </div>
              </form>
            )}
          </div>
          <button
            type="button"
            className="modal-backdrop"
            aria-label={t("common.close", { defaultValue: "Close" })}
            onClick={closeModal}
          />
        </div>
      )}
    </div>
  );
}
