import { useEffect, useMemo, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { useWorkingRepository } from "@/features/settings/hooks/useWorkingRepository";
import {
  useCloudProviders,
  useConnectICloud,
  useVerifyICloud2FA,
  useTriggerSync,
  useDisconnectCloud,
  useCreateRepository,
  useDeleteRepository,
} from "../../hooks/useCloudSync";
import {
  CloudIcon,
  KeyRoundIcon,
  InfoIcon,
  CheckCircle2Icon,
  PlayIcon,
  Trash2Icon,
  AlertTriangleIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  DatabaseIcon,
  LogOutIcon,
} from "lucide-react";

export default function CloudSettings() {
  const { t } = useI18n();
  const { repositories, repositoriesQuery } = useWorkingRepository();

  // Component States
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [domain, setDomain] = useState<"com" | "cn">("com");
  const [syncMode, setSyncMode] = useState<"import" | "one_way">("import");
  
  const [needs2FA, setNeeds2FA] = useState(false);
  const [verificationCode, setVerificationCode] = useState("");
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [successMsg, setSuccessMsg] = useState<string | null>(null);
  const [isSyncingLocal, setIsSyncingLocal] = useState(false);
  const [isDeletingRepo, setIsDeletingRepo] = useState(false);

  // React Query Hooks
  // We poll the status every 3 seconds if we connected and are actively syncing
  const { data: providersData, refetch: refetchProviders, isLoading: isLoadingProviders } = useCloudProviders({
    refetchInterval: isSyncingLocal ? 3000 : undefined,
  });

  const connectMutation = useConnectICloud();
  const verify2FAMutation = useVerifyICloud2FA();
  const triggerSyncMutation = useTriggerSync();
  const disconnectMutation = useDisconnectCloud();
  const createRepoMutation = useCreateRepository();
  const deleteRepoMutation = useDeleteRepository();

  // Find iCloud provider status
  const iCloudProvider = useMemo(() => {
    return providersData?.data?.providers?.find((p) => p.provider === "icloud");
  }, [providersData]);

  // Find designated iCloud Repository
  const iCloudRepo = useMemo(() => {
    return repositories.find((r) => r.name === "iCloud Sync");
  }, [repositories]);

  // Keep local syncing state synced with backend synced file counts
  useEffect(() => {
    if (iCloudProvider?.connected && isSyncingLocal) {
      // Simple timeout check or if user stops it. We can auto stop spinner after some idle or keep it.
      // Usually, backend background sync finishes, but synced_file_count might stop growing.
      // We keep it syncing as long as the user hasn't refreshed or we can let it run.
    }
  }, [iCloudProvider, isSyncingLocal]);

  // Action: Connect iCloud
  const handleConnect = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setSuccessMsg(null);

    if (!username || !password) {
      setErrorMsg("Apple ID and Password are required");
      return;
    }

    try {
      const res = await connectMutation.mutateAsync({
        body: {
          username,
          password,
          domain,
          syncMode,
        },
      });

      if (res.data?.needs_2fa) {
        setNeeds2FA(true);
      } else {
        setSuccessMsg("Successfully connected to iCloud!");
        refetchProviders();
      }
    } catch (err: any) {
      setErrorMsg(err?.message || "Failed to connect to iCloud. Please check your credentials.");
    }
  };

  // Action: Verify 2FA
  const handleVerify2FA = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMsg(null);
    setSuccessMsg(null);

    if (verificationCode.length !== 6) {
      setErrorMsg("Please enter a valid 6-digit verification code");
      return;
    }

    try {
      await verify2FAMutation.mutateAsync({
        body: {
          code: verificationCode,
        },
      });
      setNeeds2FA(false);
      setVerificationCode("");
      setSuccessMsg("2FA verified! iCloud is now connected.");
      refetchProviders();
    } catch (err: any) {
      setErrorMsg(err?.message || "Invalid 2FA code. Please try again.");
    }
  };

  // Action: Start Sync (with silent repo creation)
  const handleStartSync = async () => {
    setErrorMsg(null);
    setSuccessMsg(null);
    setIsSyncingLocal(true);

    try {
      let targetRepoId = iCloudRepo?.id;

      // 1. If repo "iCloud Sync" doesn't exist, create it silently
      if (!targetRepoId) {
        const repoRes = await createRepoMutation.mutateAsync({
          body: {
            name: "iCloud Sync",
          },
        });
        targetRepoId = repoRes.data?.repository?.id;
        if (!targetRepoId) {
          throw new Error("Failed to create isolated repository for iCloud Sync");
        }
      }

      // 2. Trigger iCloud sync
      await triggerSyncMutation.mutateAsync({
        body: {
          provider: "icloud",
          repository_id: targetRepoId,
        },
      });

      setSuccessMsg("iCloud synchronization started in the background!");
    } catch (err: any) {
      setErrorMsg(err?.message || "Failed to trigger iCloud sync.");
      setIsSyncingLocal(false);
    }
  };

  // Action: Disconnect
  const handleDisconnect = async () => {
    if (!confirm("Are you sure you want to disconnect your iCloud account? Active downloads will be canceled.")) {
      return;
    }

    setErrorMsg(null);
    setSuccessMsg(null);
    setIsSyncingLocal(false);

    try {
      await disconnectMutation.mutateAsync({
        params: {
          provider: "icloud",
        },
      });
      setSuccessMsg("iCloud account disconnected successfully.");
      refetchProviders();
    } catch (err: any) {
      setErrorMsg(err?.message || "Failed to disconnect iCloud.");
    }
  };

  // Action: Safe Delete iCloud Repository (Danger Zone)
  const handleDeleteICloudRepo = async () => {
    if (!iCloudRepo) return;
    
    const confirmMessage = 
      "WARNING: This will permanently delete the 'iCloud Sync' repository, " +
      "removing all downloaded photos and database records from this host. " +
      "This action CANNOT be undone. Are you absolutely sure?";
      
    if (!confirm(confirmMessage)) return;

    setErrorMsg(null);
    setSuccessMsg(null);
    setIsDeletingRepo(true);

    try {
      await deleteRepoMutation.mutateAsync({
        params: {
          id: iCloudRepo.id,
        },
      });
      setSuccessMsg("iCloud Sync repository deleted and storage cleared.");
    } catch (err: any) {
      setErrorMsg(err?.message || "Failed to delete iCloud repository.");
    } finally {
      setIsDeletingRepo(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <header className="space-y-2">
        <div className="flex items-center gap-2">
          <div className="p-2 rounded-xl bg-primary/10 text-primary">
            <CloudIcon className="size-6" />
          </div>
          <h2 className="text-2xl font-bold">{t("settings.cloud.title", { defaultValue: "Cloud Sync" })}</h2>
        </div>
        <p className="text-base-content/70">
          Sync your photos directly from cloud providers (iCloud) into an isolated repository.
        </p>
      </header>

      {/* Messages */}
      {errorMsg && (
        <div className="alert alert-error rounded-xl shadow-sm">
          <AlertTriangleIcon className="size-5" />
          <span>{errorMsg}</span>
        </div>
      )}
      {successMsg && (
        <div className="alert alert-success rounded-xl shadow-sm text-success-content">
          <CheckCircle2Icon className="size-5" />
          <span>{successMsg}</span>
        </div>
      )}

      {isLoadingProviders || repositoriesQuery.isLoading ? (
        <div className="flex flex-col items-center justify-center p-12 space-y-4">
          <span className="loading loading-ring loading-lg text-primary"></span>
          <p className="text-sm opacity-70">Loading Cloud Integration settings...</p>
        </div>
      ) : !iCloudProvider?.connected ? (
        /* CONNECT CARD (Not Logged In) */
        <div className="grid gap-6 lg:grid-cols-3">
          {/* Main login card */}
          <div className="lg:col-span-2 rounded-2xl border border-base-300 bg-base-100 p-6 shadow-sm space-y-6">
            <div className="flex items-center gap-3">
              <KeyRoundIcon className="size-5 text-primary" />
              <h3 className="text-lg font-bold">Connect iCloud Account</h3>
            </div>

            <form onSubmit={handleConnect} className="space-y-4 max-w-xl">
              <label className="form-control w-full">
                <span className="label-text font-semibold mb-1">Apple ID (Email)</span>
                <input
                  type="email"
                  placeholder="your.email@icloud.com"
                  className="input input-bordered w-full rounded-xl"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  disabled={connectMutation.isPending || needs2FA}
                />
              </label>

              <label className="form-control w-full">
                <span className="label-text font-semibold mb-1">App-Specific Password</span>
                <input
                  type="password"
                  placeholder="••••-••••-••••-••••"
                  className="input input-bordered w-full rounded-xl font-mono"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  disabled={connectMutation.isPending || needs2FA}
                />
              </label>

              <div className="grid grid-cols-2 gap-4">
                <label className="form-control w-full">
                  <span className="label-text font-semibold mb-1">Apple Domain</span>
                  <select
                    className="select select-bordered w-full rounded-xl"
                    value={domain}
                    onChange={(e) => setDomain(e.target.value as "com" | "cn")}
                    disabled={connectMutation.isPending || needs2FA}
                  >
                    <option value="com">Global (Apple.com)</option>
                    <option value="cn">Mainland China (Apple.cn / GCBD)</option>
                  </select>
                </label>

                <label className="form-control w-full">
                  <span className="label-text font-semibold mb-1">Sync Strategy</span>
                  <select
                    className="select select-bordered w-full rounded-xl"
                    value={syncMode}
                    onChange={(e) => setSyncMode(e.target.value as "import" | "one_way")}
                    disabled={connectMutation.isPending || needs2FA}
                  >
                    <option value="import">Import Only (Leave iCloud intact)</option>
                    <option value="one_way">One-Way Sync (Dynamic offset)</option>
                  </select>
                </label>
              </div>

              <button
                type="submit"
                className="btn btn-primary w-full rounded-xl mt-4"
                disabled={connectMutation.isPending || needs2FA}
              >
                {connectMutation.isPending ? (
                  <>
                    <span className="loading loading-spinner loading-xs"></span>
                    Initializing Secure Handshake...
                  </>
                ) : (
                  "Authenticate with Apple"
                )}
              </button>
            </form>
          </div>

          {/* Guidelines Sidebar */}
          <div className="rounded-2xl bg-base-200/50 p-6 space-y-4 border border-base-300">
            <div className="flex items-center gap-2 font-bold text-base-content">
              <InfoIcon className="size-5 text-primary" />
              <span>Apple Security Best Practices</span>
            </div>
            <p className="text-sm opacity-80 leading-relaxed">
              Lumilio-Photos connects directly to Apple Servers to retrieve photos. To protect your Apple Account, you <strong>must</strong> use an <strong>App-Specific Password</strong>.
            </p>
            <div className="text-xs space-y-2 opacity-85">
              <p>How to generate an App-Specific Password:</p>
              <ol className="list-decimal list-inside space-y-1 ml-1">
                <li>Go to <a href="https://appleid.apple.com" target="_blank" rel="noopener noreferrer" className="link link-primary">appleid.apple.com</a></li>
                <li>Sign in and go to <strong>Sign-In and Security</strong>.</li>
                <li>Select <strong>App-Specific Passwords</strong> and click generate.</li>
                <li>Enter "Lumilio Photos" as the label.</li>
              </ol>
            </div>
          </div>
        </div>
      ) : (
        /* DASHBOARD CARD (Logged In) */
        <div className="space-y-6">
          <div className="rounded-2xl border border-base-300 bg-base-100 p-6 shadow-sm space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="p-3 bg-emerald-500/10 text-emerald-500 rounded-2xl">
                  <ShieldCheckIcon className="size-6" />
                </div>
                <div>
                  <h3 className="text-lg font-bold">iCloud Integration Connected</h3>
                  <p className="text-sm opacity-70">
                    Active sync session established securely.
                  </p>
                </div>
              </div>

              <button
                onClick={handleDisconnect}
                className="btn btn-outline btn-error rounded-xl gap-2 self-start sm:self-center"
                disabled={disconnectMutation.isPending}
              >
                <LogOutIcon className="size-4" />
                Disconnect Account
              </button>
            </div>

            <div className="divider"></div>

            {/* Sync Status Stats */}
            <div className="grid gap-4 sm:grid-cols-3">
              <div className="bg-base-200/50 rounded-2xl p-4 border border-base-300 space-y-1">
                <span className="text-xs opacity-75 font-semibold uppercase tracking-wider">Sync Mode</span>
                <p className="text-lg font-bold capitalize">
                  {iCloudProvider.sync_mode === "one_way" ? "One-Way Sync" : "Import Only"}
                </p>
              </div>

              <div className="bg-base-200/50 rounded-2xl p-4 border border-base-300 space-y-1">
                <span className="text-xs opacity-75 font-semibold uppercase tracking-wider">Synced Items</span>
                <div className="flex items-center gap-2">
                  <p className="text-2xl font-bold font-mono">
                    {iCloudProvider.synced_file_count}
                  </p>
                  {isSyncingLocal && (
                    <RefreshCwIcon className="size-4 animate-spin text-primary" />
                  )}
                </div>
              </div>

              <div className="bg-base-200/50 rounded-2xl p-4 border border-base-300 space-y-1">
                <span className="text-xs opacity-75 font-semibold uppercase tracking-wider">Target Library</span>
                <div className="flex items-center gap-1.5 text-base-content/90 font-medium">
                  <DatabaseIcon className="size-4 text-primary" />
                  <span>{iCloudRepo ? "iCloud Sync Repository" : "Pending Creation"}</span>
                </div>
              </div>
            </div>

            {/* Sync Action Area */}
            <div className="flex items-center gap-4 pt-2">
              <button
                onClick={handleStartSync}
                className={`btn btn-primary px-8 rounded-xl gap-2 ${isSyncingLocal ? "btn-disabled" : ""}`}
                disabled={triggerSyncMutation.isPending || isSyncingLocal}
              >
                {triggerSyncMutation.isPending ? (
                  <span className="loading loading-spinner loading-xs"></span>
                ) : (
                  <PlayIcon className="size-4 fill-current" />
                )}
                {isSyncingLocal ? "Synchronizing..." : "Sync Now"}
              </button>
              
              {isSyncingLocal && (
                <div className="text-sm text-base-content/75 flex items-center gap-2">
                  <span className="size-2 rounded-full bg-primary animate-ping"></span>
                  <span>Sync running in background. Synced item counts will update automatically.</span>
                </div>
              )}
            </div>
          </div>

          {/* DANGER ZONE */}
          {iCloudRepo && (
            <div className="rounded-2xl border border-red-500/30 bg-red-500/5 p-6 space-y-4">
              <div className="flex items-center gap-2 text-red-500 font-bold">
                <AlertTriangleIcon className="size-5" />
                <h4>Danger Zone</h4>
              </div>
              <p className="text-sm opacity-80 max-w-2xl">
                You can physically purge the localized copy of your iCloud library from this server. 
                This will delete the <strong>"iCloud Sync" repository</strong>, including all downloaded media assets 
                and database metadata records. No cloud data on Apple servers will be affected.
              </p>
              
              <button
                onClick={handleDeleteICloudRepo}
                className="btn btn-error btn-outline rounded-xl gap-2"
                disabled={isDeletingRepo}
              >
                {isDeletingRepo ? (
                  <span className="loading loading-spinner loading-xs"></span>
                ) : (
                  <Trash2Icon className="size-4" />
                )}
                Delete iCloud Sync Repository
              </button>
            </div>
          )}
        </div>
      )}

      {/* 2FA Verification Modal */}
      {needs2FA && (
        <div className="modal modal-open backdrop-blur-sm">
          <div className="modal-box rounded-2xl max-w-md border border-base-300 shadow-xl space-y-6">
            <div className="text-center space-y-2">
              <div className="mx-auto size-12 rounded-full bg-primary/10 flex items-center justify-center text-primary">
                <ShieldCheckIcon className="size-6" />
              </div>
              <h3 className="text-xl font-bold">Apple ID Two-Factor Authentication</h3>
              <p className="text-sm opacity-75">
                A verification code has been sent to your Apple devices. Please enter the 6-digit code below to establish trust.
              </p>
            </div>

            {errorMsg && (
              <div className="alert alert-error rounded-xl text-xs py-2">
                <span>{errorMsg}</span>
              </div>
            )}

            <form onSubmit={handleVerify2FA} className="space-y-4">
              <label className="form-control w-full">
                <input
                  type="text"
                  maxLength={6}
                  placeholder="000000"
                  className="input input-bordered w-full text-center text-2xl font-bold tracking-widest rounded-xl font-mono"
                  value={verificationCode}
                  onChange={(e) => setVerificationCode(e.target.value.replace(/\D/g, ""))}
                  disabled={verify2FAMutation.isPending}
                  autoFocus
                />
              </label>

              <div className="flex gap-3 justify-end pt-2">
                <button
                  type="button"
                  className="btn btn-ghost rounded-xl"
                  onClick={() => {
                    setNeeds2FA(false);
                    setVerificationCode("");
                    setErrorMsg(null);
                  }}
                  disabled={verify2FAMutation.isPending}
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="btn btn-primary rounded-xl px-6"
                  disabled={verify2FAMutation.isPending || verificationCode.length !== 6}
                >
                  {verify2FAMutation.isPending ? (
                    <span className="loading loading-spinner loading-xs"></span>
                  ) : (
                    "Verify Code"
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
