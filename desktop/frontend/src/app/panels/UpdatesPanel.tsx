import { RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { UpdateService } from "../../../bindings/desktop/internal/control/index.js";
import {
  type DesktopSnapshot,
  type OperationReceipt,
} from "../../../bindings/desktop/internal/control/dto/models.js";
import { type ToastInput } from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton } from "@/components/motion/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/motion/select";
import { PageHeading, SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";
import { ActionNotice, RowActions, useTrackedOperation } from "../shared";

export function UpdatesPanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const update = snapshot.update;
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("updates.actionFailed"), description: message, status: "error" }),
  });
  const state = operation.state;

  const invoke = async (action: "check" | "download" | "apply") => {
    operation.begin();
    const requestID = `update-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "check") receipt = await UpdateService.Check(requestID, update.version);
      else if (action === "download")
        receipt = await UpdateService.Download(requestID, update.version);
      else receipt = await UpdateService.RestartAndApply(requestID, update.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };

  return (
    <>
      <PageHeading title={t("dock.updates")} description={t("updates.description")} />
      <SettingsSection title={t("dock.updates")}>
        <SettingRow
          title={t("updates.currentVersion")}
          description={t("updates.currentVersionDescription")}
        >
          <span className="setting-value tabular-value">
            {update.currentVersion || t("common.unknown")}
          </span>
        </SettingRow>
        <SettingRow title={t("updates.channel")} description={t("updates.channelDescription")}>
          {draft.preferences ? (
            <Select
              value={draft.preferences.updateChannel}
              onValueChange={(value) => draft.updatePreference("updateChannel", value)}
              disabled={draft.phase === "saving"}
              className="compact-select"
            >
              <SelectTrigger>
                <SelectValue placeholder={t("updates.channel")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="stable">{t("updates.stable")}</SelectItem>
                <SelectItem value="beta">{t("updates.beta")}</SelectItem>
              </SelectContent>
            </Select>
          ) : (
            <span className="setting-value">{t("common.loadingEllipsis")}</span>
          )}
        </SettingRow>
        <SettingRow
          title={t("updates.checkTitle")}
          description={
            update.providerAvailable ? t("updates.checkDescription") : t("updates.noProvider")
          }
        >
          <StatefulButton
            variant="secondary"
            size="sm"
            state={state}
            disabled={!update.providerAvailable}
            loadingText={t("updates.checking")}
            successText={t("updates.checked")}
            icon={<RefreshCw className="size-3.5" />}
            onClick={() => void invoke("check")}
          >
            {update.providerAvailable ? t("updates.checkNow") : t("onboarding.unavailable")}
          </StatefulButton>
        </SettingRow>
        {update.availableVersion ? (
          <SettingRow
            title={t("updates.available")}
            description={t("updates.availableDescription", { version: update.availableVersion })}
          >
            <RowActions>
              <Button
                variant="secondary"
                size="sm"
                disabled={state === "loading"}
                onClick={() => void invoke("download")}
              >
                {t("updates.download")}
              </Button>
              {update.canApply ? (
                <Button
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("apply")}
                >
                  {t("updates.restartInstall")}
                </Button>
              ) : null}
            </RowActions>
          </SettingRow>
        ) : null}
      </SettingsSection>
      {update.error?.code ? (
        <ActionNotice
          component={t("dock.updates")}
          message={update.error.message}
          actionLabel={update.providerAvailable ? t("common.retry") : undefined}
          onAction={update.providerAvailable ? () => void invoke("check") : undefined}
        />
      ) : null}
    </>
  );
}
