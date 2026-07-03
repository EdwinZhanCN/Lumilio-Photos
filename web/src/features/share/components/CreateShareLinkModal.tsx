import { useEffect, useState, type ReactNode } from "react";
import { Check, Copy, Link2, Share2 } from "lucide-react";
import Modal from "@/components/Modal";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useShareLinks, type CreateShareLinkResponseDTO } from "../hooks/useShareLinks";
import { shareUrls } from "../utils/shareUrls";

export type ShareSourceKind = "asset_snapshot" | "album" | "person" | "utility_query" | "pin";

export interface CreateShareLinkModalProps {
  open: boolean;
  onClose: () => void;
  sourceKind: ShareSourceKind;
  /** Required for sourceKind "asset_snapshot"; ignored otherwise. */
  assetIds?: string[];
  /** Required for album/person/utility_query/pin sources (backend resolves the snapshot). */
  sourceRef?: string;
  defaultTitle?: string;
  /** Called once after a link is successfully created. */
  onCreated?: (link: CreateShareLinkResponseDTO) => void;
}

const EXPIRY_PRESETS = [7, 30, 90] as const;

/**
 * Create a share link, then show its URL exactly once. Tokens are stored
 * hash-only server-side, so this modal's success state is the only place the
 * raw URL is ever shown/copyable — it cannot be recovered later from the
 * Shared Links management page.
 */
export function CreateShareLinkModal({
  open,
  onClose,
  sourceKind,
  assetIds,
  sourceRef,
  defaultTitle,
  onCreated,
}: CreateShareLinkModalProps): ReactNode {
  const { t } = useI18n();
  const showMessage = useMessage();
  const { createShareLink, isCreating } = useShareLinks();

  const [title, setTitle] = useState("");
  const [expiresInDays, setExpiresInDays] = useState<number>(30);
  const [allowDownload, setAllowDownload] = useState(false);
  const [includeOriginals, setIncludeOriginals] = useState(false);
  const [created, setCreated] = useState<CreateShareLinkResponseDTO | null>(null);
  const [copied, setCopied] = useState(false);

  const seedKey = open ? `${sourceKind}:${sourceRef ?? ""}:${assetIds?.length ?? 0}` : "closed";
  useEffect(() => {
    if (!open) return;
    setTitle(defaultTitle ?? "");
    setExpiresInDays(30);
    setAllowDownload(false);
    setIncludeOriginals(false);
    setCreated(null);
    setCopied(false);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [seedKey]);

  const close = () => {
    if (isCreating) return;
    onClose();
  };

  const canSubmit = title.trim().length > 0 && !isCreating;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit) return;
    try {
      const link = await createShareLink({
        title: title.trim(),
        source_kind: sourceKind,
        source_ref: sourceRef,
        asset_ids: sourceKind === "asset_snapshot" ? assetIds : undefined,
        expires_in_days: expiresInDays,
        allow_download: allowDownload,
        include_originals: allowDownload && includeOriginals,
      });
      setCreated(link);
      onCreated?.(link);
    } catch (error) {
      console.error("Failed to create share link:", error);
      showMessage("error", t("share.create.error", "Failed to create share link."));
    }
  };

  const handleCopy = async () => {
    if (!created?.token) return;
    try {
      await navigator.clipboard.writeText(shareUrls.publicShareUrl(created.token));
      setCopied(true);
      showMessage("success", t("share.create.copied", "Link copied to clipboard."));
    } catch (error) {
      console.error("Failed to copy share link:", error);
    }
  };

  const footer = created ? (
    <button type="button" className="btn btn-primary shadow-none" onClick={onClose}>
      {t("common.done", "Done")}
    </button>
  ) : (
    <>
      <button
        type="button"
        className="btn btn-ghost shadow-none"
        onClick={close}
        disabled={isCreating}
      >
        {t("common.cancel")}
      </button>
      <button
        type="submit"
        form="create-share-link-form"
        className="btn btn-primary shadow-none"
        disabled={!canSubmit}
      >
        {isCreating && <span className="loading loading-spinner loading-sm" />}
        {isCreating
          ? t("share.create.creating", "Creating…")
          : t("share.create.submit", "Create link")}
      </button>
    </>
  );

  return (
    <Modal
      open={open}
      onClose={close}
      size="sm"
      icon={<Share2 size={20} />}
      title={created ? t("share.create.createdTitle", "Link created") : t("share.create.title", "Share")}
      footer={footer}
    >
      {created && created.token ? (
        <div className="space-y-4 p-6">
          <p className="text-sm text-base-content/70">
            {t(
              "share.create.createdHint",
              "Copy this link now — it won't be shown again after you close this dialog.",
            )}
          </p>
          <div className="flex items-center gap-2 rounded-lg border border-base-300 bg-base-200/40 px-3 py-2">
            <Link2 className="size-4 shrink-0 text-base-content/45" />
            <input
              type="text"
              readOnly
              value={shareUrls.publicShareUrl(created.token)}
              className="min-w-0 flex-1 truncate bg-transparent text-sm outline-none"
              onFocus={(e) => e.currentTarget.select()}
            />
            <button
              type="button"
              className="btn btn-ghost btn-sm gap-1.5 shadow-none"
              onClick={handleCopy}
            >
              {copied ? <Check className="size-4 text-success" /> : <Copy className="size-4" />}
              {copied ? t("common.copied", "Copied") : t("common.copy", "Copy")}
            </button>
          </div>
        </div>
      ) : (
        <form id="create-share-link-form" onSubmit={handleSubmit} className="space-y-5 p-6">
          <fieldset className="fieldset w-full py-0">
            <legend className="fieldset-legend pb-1 text-xs font-semibold uppercase tracking-wide text-base-content/55">
              {t("share.create.fields.title.label", "Title")}
            </legend>
            <input
              type="text"
              placeholder={t("share.create.fields.title.placeholder", "Shared with family")}
              className="input input-bordered w-full"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              required
            />
          </fieldset>

          <fieldset className="fieldset w-full py-0">
            <legend className="fieldset-legend pb-1 text-xs font-semibold uppercase tracking-wide text-base-content/55">
              {t("share.create.fields.expiry.label", "Expires in")}
            </legend>
            <div className="join">
              {EXPIRY_PRESETS.map((days) => (
                <button
                  key={days}
                  type="button"
                  className={`btn join-item btn-sm ${expiresInDays === days ? "btn-primary" : "btn-outline"}`}
                  onClick={() => setExpiresInDays(days)}
                >
                  {t("share.create.fields.expiry.days", { count: days, defaultValue: "{{count}} days" })}
                </button>
              ))}
            </div>
          </fieldset>

          <div className="space-y-3 border-t border-base-200 pt-4">
            <label className="flex cursor-pointer items-center justify-between gap-3">
              <span className="text-sm">
                {t("share.create.fields.allowDownload.label", "Allow download")}
              </span>
              <input
                type="checkbox"
                className="toggle toggle-primary"
                checked={allowDownload}
                onChange={(e) => setAllowDownload(e.target.checked)}
              />
            </label>
            {allowDownload && (
              <label className="flex cursor-pointer items-center justify-between gap-3">
                <span className="text-sm">
                  {t("share.create.fields.includeOriginals.label", "Include originals")}
                </span>
                <input
                  type="checkbox"
                  className="toggle toggle-primary"
                  checked={includeOriginals}
                  onChange={(e) => setIncludeOriginals(e.target.checked)}
                />
              </label>
            )}
          </div>
        </form>
      )}
    </Modal>
  );
}

export default CreateShareLinkModal;
