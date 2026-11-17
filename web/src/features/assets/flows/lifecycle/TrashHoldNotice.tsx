import { useState } from "react";
import { ShieldCheck } from "lucide-react";
import { Modal } from "@/components/ui/Modal";
import { useMessage } from "@/features/notifications";
import { useAssetLifecycle } from "@/lib/assets/useAssetLifecycle";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n";

/** Held Trash in the caller's scope, for the notice and the bulk action. */
export function useTrashHolds(repositoryId?: string) {
  return $api.useQuery(
    "get",
    "/api/v1/assets/trash-holds",
    { params: { query: repositoryId ? { repository_id: repositoryId } : {} } },
    { staleTime: 10_000 },
  );
}

/**
 * Tells the user that some trashed files are inherited (a Repository opened,
 * copied, or restored into this catalog) and therefore held out of automatic
 * deletion until they decide. With a Repository in scope the decision can be
 * taken for every held file at once; otherwise the selection action applies.
 */
export default function TrashHoldNotice({ repositoryId }: { repositoryId?: string }) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const holds = useTrashHolds(repositoryId);
  const lifecycle = useAssetLifecycle();
  const [confirming, setConfirming] = useState(false);
  const count = holds.data?.count ?? 0;
  if (count === 0) return null;

  const acceptAll = async () => {
    if (!repositoryId || lifecycle.acceptRetention.isPending) return;
    try {
      const result = await lifecycle.acceptRetention.mutateAsync({
        body: { repository_id: repositoryId, confirm: true },
      });
      setConfirming(false);
      showMessage(
        "success",
        t("assets.trash.holds.accepted", "{{count}} files now follow the normal retention.", {
          count: result.entries ?? 0,
        }),
      );
    } catch (error) {
      showMessage(
        "error",
        localizeAPIProblem(
          error,
          t,
          t("assets.lifecycle.actionFailed", "The lifecycle action could not be completed."),
        ),
      );
    }
  };

  return (
    <div role="status" className="alert alert-info alert-soft mx-4 mt-3 py-2 text-sm">
      <ShieldCheck className="size-4 shrink-0" aria-hidden />
      <div className="min-w-0 flex-1">
        <p className="font-medium">
          {t("assets.trash.holds.title", "{{count}} files are kept until you decide", { count })}
        </p>
        <p className="text-xs text-base-content/70">
          {t(
            "assets.trash.holds.help",
            "They were in the Trash before this Repository was opened here, so they never expire on their own. Restore them, delete them permanently, or keep them with the normal retention.",
          )}
        </p>
      </div>
      {repositoryId ? (
        <button type="button" className="btn btn-sm" onClick={() => setConfirming(true)}>
          {t("assets.trash.holds.acceptAll", "Keep with normal retention")}
        </button>
      ) : null}
      <Modal
        open={confirming}
        onClose={() => setConfirming(false)}
        dismissable={!lifecycle.acceptRetention.isPending}
        size="sm"
        title={t("assets.trash.holds.acceptAll", "Keep with normal retention")}
        footer={
          <>
            <button
              type="button"
              className="btn btn-ghost"
              disabled={lifecycle.acceptRetention.isPending}
              onClick={() => setConfirming(false)}
            >
              {t("common.cancel", "Cancel")}
            </button>
            <button
              type="button"
              className="btn btn-primary"
              disabled={lifecycle.acceptRetention.isPending}
              onClick={() => void acceptAll()}
            >
              {t("common.confirm", "Confirm")}
            </button>
          </>
        }
      >
        <p>
          {t(
            "assets.trash.holds.acceptWarning",
            "The held files in this Repository will be deleted permanently when the normal retention runs out, counted from now. You can still restore them until then.",
          )}
        </p>
      </Modal>
    </div>
  );
}
