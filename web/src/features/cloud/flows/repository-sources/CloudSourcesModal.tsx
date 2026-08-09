import { useMemo, useState, type FormEvent } from "react";
import { Cloud, CloudDownload, Pause, Play, Plus, X } from "lucide-react";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n";
import { useCloudCredentials } from "../../api/useCloudCredentials";
import {
  useBindRepositoryCloudSource,
  useCancelCloudImport,
  useRepositoryCloudStatus,
  useResumeCloudImport,
  useStartRepositoryCloudImport,
} from "../../api/useRepositoryCloud";

export default function CloudSourcesModal({
  repositoryId,
  repositoryName,
  isOpen,
  onClose,
}: {
  repositoryId: string;
  repositoryName: string;
  isOpen: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const statusQuery = useRepositoryCloudStatus(repositoryId, isOpen);
  const credentialsQuery = useCloudCredentials();
  const bindMutation = useBindRepositoryCloudSource();
  const startMutation = useStartRepositoryCloudImport();
  const cancelMutation = useCancelCloudImport();
  const resumeMutation = useResumeCloudImport();
  const [credentialId, setCredentialId] = useState("");
  const [remoteAlbum, setRemoteAlbum] = useState("");
  const connectedCredentials = useMemo(
    () => (credentialsQuery.data?.credentials ?? []).filter((item) => item.status === "connected"),
    [credentialsQuery.data?.credentials],
  );
  const sources = statusQuery.data?.sources ?? [];
  const busy =
    bindMutation.isPending ||
    startMutation.isPending ||
    cancelMutation.isPending ||
    resumeMutation.isPending;

  if (!isOpen) return null;

  const bind = async (event: FormEvent) => {
    event.preventDefault();
    if (!credentialId || busy) return;
    try {
      await bindMutation.mutateAsync({
        params: { path: { id: repositoryId } },
        body: {
          credential_id: credentialId,
          remote_scope: remoteAlbum.trim() ? { album: remoteAlbum.trim() } : {},
        },
      });
      setCredentialId("");
      setRemoteAlbum("");
      showMessage(
        "success",
        t("cloud.sources.bound", "Cloud source connected and its first import started."),
      );
    } catch (error) {
      showMessage(
        "error",
        error instanceof Error
          ? error.message
          : t("cloud.sources.bindFailed", "Unable to connect cloud source."),
      );
    }
  };

  return (
    <div className="modal modal-open z-modal">
      <div className="modal-box max-w-2xl">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h3 className="flex items-center gap-2 text-base font-semibold">
              <Cloud size={18} />
              {t("cloud.sources.title", "Cloud sources")}
            </h3>
            <p className="mt-1 text-sm text-base-content/60">{repositoryName}</p>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
            disabled={busy}
            aria-label={t("common.close", { defaultValue: "Close" })}
          >
            <X size={18} />
          </button>
        </div>

        <div className="mt-5 space-y-3">
          {sources.map((source) => {
            const credential = source.credential;
            if (!credential?.id) return null;
            const run = source.latest_run;
            const runId = run?.id;
            const running =
              run?.status === "queued" || run?.status === "running" || run?.status === "cancelling";
            const resumable =
              run?.status !== undefined &&
              ["cancelled", "failed", "interrupted"].includes(run.status);
            return (
              <section key={credential.id} className="rounded-lg border border-base-300 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <div className="font-medium">{credential.display_name}</div>
                    <div className="text-xs text-base-content/55">
                      {credential.provider_title} · {credential.masked_identity}
                    </div>
                    {source.remote_scope?.album ? (
                      <div className="mt-1 font-mono text-xs text-base-content/60">
                        {t("cloud.sources.scope", "Remote scope")}: {source.remote_scope.album}
                      </div>
                    ) : null}
                    {run ? (
                      <div className="mt-2 text-xs text-base-content/65">
                        {run.status} · {(run.imported_count ?? 0).toLocaleString()}{" "}
                        {t("cloud.sources.imported", "imported")} ·{" "}
                        {(run.failed_count ?? 0).toLocaleString()}{" "}
                        {t("cloud.sources.failed", "failed")}
                      </div>
                    ) : null}
                  </div>
                  <div className="flex gap-2">
                    {running && runId ? (
                      <button
                        type="button"
                        className="btn btn-sm btn-warning btn-soft"
                        disabled={busy}
                        onClick={() =>
                          void cancelMutation.mutateAsync({ params: { path: { id: runId } } })
                        }
                      >
                        <Pause size={14} /> {t("cloud.sources.cancel", "Cancel")}
                      </button>
                    ) : resumable && runId ? (
                      <button
                        type="button"
                        className="btn btn-sm btn-primary"
                        disabled={busy}
                        onClick={() =>
                          void resumeMutation.mutateAsync({ params: { path: { id: runId } } })
                        }
                      >
                        <Play size={14} /> {t("cloud.sources.resume", "Resume")}
                      </button>
                    ) : (
                      <button
                        type="button"
                        className="btn btn-sm btn-soft"
                        disabled={busy}
                        onClick={() =>
                          void startMutation.mutateAsync({
                            params: { path: { id: repositoryId } },
                            body: { credential_id: credential.id },
                          })
                        }
                      >
                        <CloudDownload size={14} /> {t("cloud.sources.importNow", "Import now")}
                      </button>
                    )}
                  </div>
                </div>
              </section>
            );
          })}
          {!statusQuery.isLoading && sources.length === 0 ? (
            <p className="rounded-lg bg-base-200/60 p-4 text-sm text-base-content/65">
              {t("cloud.sources.empty", "No cloud source is connected to this repository.")}
            </p>
          ) : null}
        </div>

        <form
          className="mt-6 space-y-3 border-t border-base-200 pt-5"
          onSubmit={(event) => void bind(event)}
        >
          <h4 className="text-sm font-semibold">
            {t("cloud.sources.add", "Connect another source")}
          </h4>
          <label className="fieldset gap-1">
            <span className="fieldset-legend p-0 text-sm">
              {t("cloud.sources.credential", "Cloud account")}
            </span>
            <select
              className="select select-bordered w-full"
              value={credentialId}
              onChange={(event) => setCredentialId(event.target.value)}
              disabled={busy}
              required
            >
              <option value="">
                {t("cloud.sources.selectCredential", "Select a connected account")}
              </option>
              {connectedCredentials.map((credential) => (
                <option key={credential.id} value={credential.id}>
                  {credential.display_name} · {credential.masked_identity}
                </option>
              ))}
            </select>
          </label>
          <label className="fieldset gap-1">
            <span className="fieldset-legend p-0 text-sm">
              {t("cloud.sources.album", "Remote album (optional)")}
            </span>
            <input
              className="input input-bordered w-full"
              value={remoteAlbum}
              onChange={(event) => setRemoteAlbum(event.target.value)}
              placeholder={t("cloud.sources.albumPlaceholder", "Favorites")}
              disabled={busy}
            />
            <span className="label text-xs text-base-content/55">
              {t(
                "cloud.sources.albumHint",
                "Leave empty for All Photos. The selected album remains fixed for resumed imports.",
              )}
            </span>
          </label>
          <button type="submit" className="btn btn-primary btn-sm" disabled={!credentialId || busy}>
            {bindMutation.isPending ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <Plus size={15} />
            )}
            {t("cloud.sources.connectAndImport", "Connect and import")}
          </button>
        </form>
      </div>
      <button
        type="button"
        className="modal-backdrop"
        onClick={onClose}
        aria-label={t("common.close", { defaultValue: "Close" })}
      />
    </div>
  );
}
