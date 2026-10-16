import { FolderOpen, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { StorageService } from "../../../bindings/desktop/internal/control/index.js";
import {
  RuntimePhase,
  type DesktopSnapshot,
  type HostActionTicket,
  type StorageShortcut,
} from "../../../bindings/desktop/internal/control/dto/models.js";
import { type ToastInput } from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton } from "@/components/motion/button";
import { PageHeading, SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";
import { errorMessage } from "@/lib/desktop/errors";
import { ActionNotice, RowActions, useTrackedOperation } from "../shared";

export function StoragePanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const [items, setItems] = useState<StorageShortcut[]>([]);
  const [hostActions, setHostActions] = useState<HostActionTicket[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [hostActionError, setHostActionError] = useState<string | null>(null);
  const [decliningActionID, setDecliningActionID] = useState<string | null>(null);

  const refresh = useCallback(async () => {
    try {
      setLoadError(null);
      setItems((await StorageService.ListShortcuts()) || []);
    } catch (reason: unknown) {
      setLoadError(errorMessage(reason));
    }
  }, []);
  const refreshHostActions = useCallback(async () => {
    try {
      setHostActionError(null);
      setHostActions((await StorageService.ListHostActions()) || []);
    } catch (reason: unknown) {
      setHostActionError(errorMessage(reason));
    }
  }, []);
  const hostOperation = useTrackedOperation(snapshot.operations, {
    onSucceeded: () => {
      void refresh();
      void refreshHostActions();
    },
    onFailed: (message) =>
      showToast({
        title: t("storage.hostActionFailed", "Request could not be completed"),
        description: message,
        status: "error",
      }),
  });

  useEffect(() => {
    void refresh();
    void refreshHostActions();
    if (snapshot.runtime.phase !== RuntimePhase.RuntimeRunning) return undefined;
    const timer = window.setInterval(() => void refreshHostActions(), 3000);
    return () => window.clearInterval(timer);
  }, [refresh, refreshHostActions, snapshot.storage.version, snapshot.runtime.phase]);

  const approveHostAction = async (action: HostActionTicket) => {
    hostOperation.begin();
    try {
      const receipt = await StorageService.ApproveHostAction(
        `host-action-${crypto.randomUUID()}`,
        snapshot.storage.version,
        action.id,
        action.nonce,
      );
      hostOperation.track(receipt);
    } catch (reason: unknown) {
      hostOperation.reject(reason);
    }
  };

  const declineHostAction = async (action: HostActionTicket) => {
    setDecliningActionID(action.id);
    try {
      await StorageService.DeclineHostAction(action.id);
      await refreshHostActions();
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setHostActionError(message);
      showToast({
        title: t("storage.hostActionDeclineFailed", "Request could not be declined"),
        description: message,
        status: "error",
      });
    } finally {
      setDecliningActionID(null);
    }
  };

  const configuredPath = draft.runtime?.storagePath || "";
  const defaultLocation =
    items.find((item) => item.kind === "default") ||
    items.find((item) => configuredPath !== "" && item.path === configuredPath);
  const additional = items.filter((item) => item.id !== defaultLocation?.id);

  return (
    <>
      <PageHeading title={t("storage.title")} description={t("storage.description")} />

      {loadError ? (
        <ActionNotice
          component={t("dock.storage")}
          message={loadError}
          actionLabel={t("common.retry")}
          onAction={() => void refresh()}
        />
      ) : null}

      <SettingsSection title={t("storage.webRequests", "Requests from Web")}>
        {hostActionError ? (
          <ActionNotice
            component={t("storage.webRequests", "Requests from Web")}
            message={hostActionError}
            actionLabel={t("common.retry")}
            onAction={() => void refreshHostActions()}
          />
        ) : null}
        {hostActions.length ? (
          hostActions.map((action) => (
            <SettingRow
              key={action.id}
              title={hostActionLabel(action.kind, t)}
              description={t(
                "storage.hostActionRequestedBy",
                "{{purpose}} · Requested by {{actor}}",
                {
                  purpose:
                    action.purpose ||
                    t(
                      "storage.hostActionDescription",
                      "A signed-in administrator requested access to a folder on this computer.",
                    ),
                  actor: action.actor,
                },
              )}
            >
              {action.status === "needs_decision" && action.riskWarnings?.length ? (
                <div className="rounded-md border border-warning/35 bg-warning/10 px-3 py-2 text-xs text-warning-content">
                  <strong>
                    {t(
                      "storage.hostActionRiskTitle",
                      "Confirm storage risks before continuing",
                    )}
                  </strong>
                  <div className="mt-1">
                    {action.riskWarnings.map((warning) => hostActionRiskLabel(warning, t)).join(" · ")}
                  </div>
                </div>
              ) : null}
              <RowActions>
                <span className="setting-value">
                  {t("storage.hostActionExpires", "Expires {{time}}", {
                    time: new Date(action.expiresAt).toLocaleTimeString(),
                  })}
                </span>
                <Button
                  variant="ghost"
                  size="sm"
                  disabled={decliningActionID === action.id || hostOperation.state === "loading"}
                  onClick={() => void declineHostAction(action)}
                >
                  <X className="size-3.5" /> {t("common.decline", "Decline")}
                </Button>
                <StatefulButton
                  variant="secondary"
                  size="sm"
                  state={hostOperation.state}
                  loadingText={
                    action.status === "needs_decision"
                      ? t("storage.hostActionConfirmingRisk", "Confirming risks")
                      : t("storage.hostActionChoosing", "Choosing folder")
                  }
                  successText={t("storage.hostActionApproved", "Approved")}
                  icon={<FolderOpen className="size-3.5" />}
                  disabled={
                    decliningActionID !== null ||
                    (action.status === "needs_decision" && !action.riskWarnings?.length)
                  }
                  onClick={() => void approveHostAction(action)}
                >
                  {action.status === "needs_decision" && !action.riskWarnings?.length
                    ? t(
                        "storage.hostActionResolveInWeb",
                        "Resolve in Web Storage admin",
                      )
                    : action.status === "needs_decision"
                      ? t("storage.confirmRiskAndContinue", "Confirm risks and continue")
                      : t("storage.reviewAndChoose", "Review and choose folder")}
                </StatefulButton>
              </RowActions>
            </SettingRow>
          ))
        ) : (
          <div className="empty-state compact-empty-state">
            <div>
              <strong>{t("storage.noWebRequests", "No pending Web requests")}</strong>
              <span>
                {t(
                  "storage.noWebRequestsHint",
                  "Native folder requests from the Web app will appear here for local approval.",
                )}
              </span>
            </div>
          </div>
        )}
      </SettingsSection>

      <SettingsSection title={t("storage.defaultLocation")}>
        <SettingRow
          title={t("storage.defaultLocation")}
          description={
            defaultLocation
              ? storageLocationDescription(defaultLocation, t)
              : configuredPath || t("storage.noDefault")
          }
        >
          <Button
            variant="secondary"
            size="sm"
            disabled={!defaultLocation?.canOpen}
            onClick={() => defaultLocation && void StorageService.OpenLocation(defaultLocation.id)}
          >
            <FolderOpen className="size-3.5" /> {t("common.open")}
          </Button>
          <Button
            variant="secondary"
            size="sm"
            disabled={draft.phase === "preparing" || draft.phase === "saving"}
            onClick={() => void draft.chooseDefaultStorage()}
          >
            {t("storage.locateDefaultLocation", "Locate Default Storage Location")}
          </Button>
        </SettingRow>
      </SettingsSection>

      <SettingsSection title={t("storage.additional")}>
        {additional.length ? (
          additional.map((item) => (
            <SettingRow
              key={item.id}
              title={item.name || t("storage.location")}
              description={storageLocationDescription(item, t)}
            >
              <RowActions>
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={!item.canOpen}
                  onClick={() => void StorageService.OpenLocation(item.id)}
                >
                  <FolderOpen className="size-3.5" /> {t("common.open")}
                </Button>
              </RowActions>
            </SettingRow>
          ))
        ) : (
          <div className="empty-state compact-empty-state">
            <div>
              <strong>{t("storage.noAdditional")}</strong>
              <span>
                {snapshot.runtime.phase === RuntimePhase.RuntimeRunning
                  ? t("storage.noAdditionalHint")
                  : t("storage.startServerHint")}
              </span>
            </div>
          </div>
        )}
      </SettingsSection>
    </>
  );
}

function hostActionLabel(kind: string, t: ReturnType<typeof useTranslation>["t"]): string {
  if (kind === "authorize_storage_location") {
    return t("storage.hostActionAddLocation", "Add Storage Location");
  }
  if (kind === "open_repository") {
    return t("storage.hostActionOpenRepository", "Open Existing Repository");
  }
  if (kind === "locate_storage_location") {
    return t("storage.hostActionLocateLocation", "Locate Storage Location");
  }
  if (kind === "locate_repository") {
    return t("storage.hostActionLocateRepository", "Locate Repository");
  }
  return t("storage.hostAction", "Storage request");
}

function hostActionRiskLabel(
  warning: string,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  if (warning === "network_filesystem") return t("storage.riskNetwork", "Network filesystem");
  if (warning === "removable_storage") return t("storage.riskRemovable", "Removable storage");
  if (warning === "cloud_sync_directory")
    return t("storage.riskCloudSync", "Cloud-sync managed folder");
  if (warning === "mount_fingerprint_changed")
    return t("storage.riskMountChanged", "Mounted filesystem changed");
  if (warning === "unavailable_cloud_placeholder")
    return t("storage.riskPlaceholder", "Files must be downloaded first");
  return warning;
}

function storageLocationDescription(
  item: StorageShortcut,
  t: ReturnType<typeof useTranslation>["t"],
): string {
  const details = [item.path || item.status];
  details.push(
    t("storage.repositoryCount", "{{count}} Repositories", { count: item.repositoryCount }),
  );
  return details.join(" · ");
}
