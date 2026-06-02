import { useMemo, useState, type FormEvent } from "react";
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  CloudIcon,
  KeyRoundIcon,
  LogOutIcon,
  PlusIcon,
  ShieldCheckIcon,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useCloudCredentials,
  useCreateICloudCredential,
  useDisableCloudCredential,
  useVerifyICloudCredential2FA,
  type CloudCredential,
} from "../../hooks/useCloudSync";

export default function CloudSettings() {
  const { t } = useI18n();
  const credentialsQuery = useCloudCredentials();
  const createCredential = useCreateICloudCredential();
  const verify2FA = useVerifyICloudCredential2FA();
  const disableCredential = useDisableCloudCredential();

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
        throw new Error("Credential response was empty.");
      }
      if (result.data?.needs_2fa) {
        setPendingCredential(credential);
        setSuccessMsg("Apple sent a verification code to your trusted devices.");
      } else {
        resetForm();
        setSuccessMsg("iCloud credential connected.");
      }
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : "Failed to create iCloud credential.");
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
      setPendingCredential(null);
      setVerificationCode("");
      resetForm();
      setSuccessMsg("iCloud credential verified.");
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : "Failed to verify iCloud 2FA code.");
    }
  };

  const handleDisable = async (credential: CloudCredential) => {
    if (!confirm(`Disable ${credential.display_name}? Existing imports stay in Lumilio.`)) {
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
      setSuccessMsg("Cloud credential disabled.");
    } catch (error) {
      setErrorMsg(error instanceof Error ? error.message : "Failed to disable credential.");
    }
  };

  return (
    <div className="space-y-6">
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <div className="rounded-lg bg-primary/10 p-2 text-primary">
            <CloudIcon className="size-6" />
          </div>
          <h2 className="text-2xl font-bold">
            {t("settings.cloud.title", { defaultValue: "Cloud Import" })}
          </h2>
        </div>
        <p className="text-sm text-base-content/65">
          Manage reusable iCloud credentials. Repositories choose one of these accounts when they
          are created from Manage.
        </p>
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

      <section className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3">
            <div>
              <h3 className="text-base font-semibold">Saved credentials</h3>
              <p className="text-sm text-base-content/60">
                {connectedCount} connected account{connectedCount === 1 ? "" : "s"}
              </p>
            </div>
            {credentialsQuery.isLoading && <span className="loading loading-spinner loading-sm" />}
          </div>

          {credentials.length === 0 && !credentialsQuery.isLoading ? (
            <div className="rounded-lg border border-base-300 px-4 py-8 text-center text-sm text-base-content/60">
              No iCloud credentials are configured.
            </div>
          ) : (
            <div className="grid gap-3">
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
                            className={`badge badge-sm ${
                              credential.status === "connected"
                                ? "badge-success"
                                : credential.status === "pending_2fa"
                                  ? "badge-warning"
                                  : "badge-ghost"
                            }`}
                          >
                            {credential.status}
                          </span>
                        </div>
                        <p className="mt-1 text-xs text-base-content/60">
                          {credential.masked_account} · iCloud {credential.domain}
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
                      Disable
                    </button>
                  </div>
                </article>
              ))}
            </div>
          )}
        </div>

        <aside className="space-y-4">
          <div className="rounded-lg border border-base-300 bg-base-100 p-4 shadow-sm">
            <div className="mb-4 flex items-center gap-2">
              <PlusIcon className="size-5 text-primary" />
              <h3 className="text-base font-semibold">Add iCloud credential</h3>
            </div>

            <form onSubmit={handleCreate} className="space-y-4">
              <label className="form-control w-full">
                <span className="label-text pb-1 text-sm font-medium">Label</span>
                <input
                  type="text"
                  className="input input-bordered w-full"
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                  placeholder="Personal iCloud"
                  disabled={createCredential.isPending || Boolean(pendingCredential)}
                />
              </label>

              <label className="form-control w-full">
                <span className="label-text pb-1 text-sm font-medium">Apple ID</span>
                <input
                  type="email"
                  className="input input-bordered w-full"
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  placeholder="you@icloud.com"
                  disabled={createCredential.isPending || Boolean(pendingCredential)}
                  required
                />
              </label>

              <label className="form-control w-full">
                <span className="label-text pb-1 text-sm font-medium">App-specific password</span>
                <input
                  type="password"
                  className="input input-bordered w-full font-mono"
                  value={password}
                  onChange={(event) => setPassword(event.target.value)}
                  placeholder="xxxx-xxxx-xxxx-xxxx"
                  disabled={createCredential.isPending || Boolean(pendingCredential)}
                  required
                />
              </label>

              <label className="form-control w-full">
                <span className="label-text pb-1 text-sm font-medium">Apple domain</span>
                <select
                  className="select select-bordered w-full"
                  value={domain}
                  onChange={(event) => setDomain(event.target.value as "com" | "cn")}
                  disabled={createCredential.isPending || Boolean(pendingCredential)}
                >
                  <option value="com">Global iCloud</option>
                  <option value="cn">Mainland China iCloud</option>
                </select>
              </label>

              <button
                type="submit"
                className="btn btn-primary w-full gap-2"
                disabled={!username.trim() || !password || createCredential.isPending || Boolean(pendingCredential)}
              >
                {createCredential.isPending ? (
                  <span className="loading loading-spinner loading-xs" />
                ) : (
                  <ShieldCheckIcon size={16} />
                )}
                Connect credential
              </button>
            </form>
          </div>

          <div className="rounded-lg border border-base-300 bg-base-200/40 p-4 text-sm text-base-content/70">
            Lumilio stores the iCloud session cookie in an isolated credential directory. The Apple
            ID password is used only during authentication and is not persisted.
          </div>
        </aside>
      </section>

      {pendingCredential && (
        <div className="modal modal-open">
          <div className="modal-box max-w-md rounded-lg">
            <div className="mb-4 text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-lg bg-primary/10 text-primary">
                <ShieldCheckIcon className="size-6" />
              </div>
              <h3 className="text-lg font-semibold">Two-factor authentication</h3>
              <p className="mt-1 text-sm text-base-content/65">
                Enter the 6-digit code sent to your Apple devices for {pendingCredential.display_name}.
              </p>
            </div>

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
                  onClick={() => {
                    setPendingCredential(null);
                    setVerificationCode("");
                  }}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn btn-primary"
                  disabled={verificationCode.length !== 6 || verify2FA.isPending}
                >
                  {verify2FA.isPending && <span className="loading loading-spinner loading-xs" />}
                  Verify
                </button>
              </div>
            </form>
          </div>
          <button
            type="button"
            className="modal-backdrop"
            aria-label="Close"
            onClick={() => setPendingCredential(null)}
          />
        </div>
      )}
    </div>
  );
}
