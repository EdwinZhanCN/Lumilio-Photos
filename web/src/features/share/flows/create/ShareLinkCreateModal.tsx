import { useEffect, useState, type ReactNode } from "react";
import { Check, Copy, Link2, Share2 } from "lucide-react";
import Modal from "@/components/ui/Modal";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import { shareUrls } from "../../model/shareUrls";

export type ShareLinkCreateValues = {
  title: string;
  description?: string;
  expiresInDays: number;
  allowDownload: boolean;
  includeOriginals: boolean;
};

type CreatedShareLink = {
  token?: string;
};

export interface ShareLinkCreateModalProps<T extends CreatedShareLink> {
  open: boolean;
  onClose: () => void;
  defaultTitle?: string;
  isCreating: boolean;
  onCreate: (values: ShareLinkCreateValues) => Promise<T>;
  onCreated?: (link: T) => void;
  errorMessage?: string;
}

const EXPIRY_PRESETS = [7, 30, 90] as const;

/** Shared share-link form and one-time token result surface. */
export function ShareLinkCreateModal<T extends CreatedShareLink>({
  open,
  onClose,
  defaultTitle,
  isCreating,
  onCreate,
  onCreated,
  errorMessage,
}: ShareLinkCreateModalProps<T>): ReactNode {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [expiresInDays, setExpiresInDays] = useState<number>(30);
  const [allowDownload, setAllowDownload] = useState(false);
  const [includeOriginals, setIncludeOriginals] = useState(false);
  const [created, setCreated] = useState<T | null>(null);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!open) return;
    setTitle(defaultTitle ?? "");
    setDescription("");
    setExpiresInDays(30);
    setAllowDownload(false);
    setIncludeOriginals(false);
    setCreated(null);
    setCopied(false);
  }, [defaultTitle, open]);

  const close = () => {
    if (!isCreating) onClose();
  };
  const canSubmit = title.trim().length > 0 && !isCreating;

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!canSubmit) return;
    try {
      const link = await onCreate({
        title: title.trim(),
        description: description.trim() || undefined,
        expiresInDays,
        allowDownload,
        includeOriginals: allowDownload && includeOriginals,
      });
      setCreated(link);
      onCreated?.(link);
    } catch (error) {
      console.error("Failed to create share link:", error);
      showMessage("error", errorMessage ?? t("share.create.error", "Failed to create share link."));
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
      showMessage("error", t("share.create.copyError", "Could not copy the link."));
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
      dismissable={!isCreating}
      icon={<Share2 size={20} />}
      title={
        created ? t("share.create.createdTitle", "Link created") : t("share.create.title", "Share")
      }
      footer={footer}
    >
      {created?.token ? (
        <div className="space-y-4 p-6">
          <p className="text-sm leading-6 text-base-content/70">
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
              onFocus={(event) => event.currentTarget.select()}
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
            <legend className="fieldset-legend pb-1 text-xs font-semibold tracking-wide text-base-content/55">
              {t("share.create.fields.title.label", "Title")}
            </legend>
            <input
              type="text"
              placeholder={t("share.create.fields.title.placeholder", "Shared with family")}
              className="input input-bordered w-full"
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              required
            />
          </fieldset>

          <fieldset className="fieldset w-full py-0">
            <legend className="fieldset-legend pb-1 text-xs font-semibold tracking-wide text-base-content/55">
              {t("share.create.fields.description.label", "Description")}
            </legend>
            <textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              placeholder={t(
                "share.create.fields.description.placeholder",
                "Add an optional note for the people receiving this link",
              )}
              className="textarea textarea-bordered min-h-20 w-full resize-y"
            />
          </fieldset>

          <fieldset className="fieldset w-full py-0">
            <legend className="fieldset-legend pb-1 text-xs font-semibold tracking-wide text-base-content/55">
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
                  {t("share.create.fields.expiry.days", {
                    count: days,
                    defaultValue: "{{count}} days",
                  })}
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
                onChange={(event) => setAllowDownload(event.target.checked)}
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
                  onChange={(event) => setIncludeOriginals(event.target.checked)}
                />
              </label>
            )}
          </div>
        </form>
      )}
    </Modal>
  );
}

export default ShareLinkCreateModal;
