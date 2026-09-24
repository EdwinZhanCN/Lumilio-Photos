import { RotateCcw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { RuntimeService } from "../../../bindings/desktop/internal/control/index.js";
import {
  RuntimePhase,
  type DesktopSnapshot,
  type OperationReceipt,
} from "../../../bindings/desktop/internal/control/dto/models.js";
import { AnimatedBadge } from "@/components/motion/animated-badge";
import { type ToastInput } from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton } from "@/components/motion/button";
import { PageHeading, SettingRow, SettingsSection } from "@/components/settings/setting-layout";
import { RuntimeConfigWorkspace } from "@/features/runtime/RuntimeConfigWorkspace";
import { type SettingsDraftController } from "@/features/settings/use-settings-draft";
import {
  ActionNotice,
  RowActions,
  SettingsLoading,
  presentationStatus,
  useTrackedOperation,
} from "../shared";

export function ServerPanel({
  snapshot,
  draft,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  draft: SettingsDraftController;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const runtime = snapshot.runtime;
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("server.actionFailed"), description: message, status: "error" }),
  });
  const actionState = operation.state;
  const actionError = operation.error;

  const invoke = async (action: "start" | "stop" | "restart" | "retry") => {
    operation.begin();
    const requestID = `runtime-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "start") receipt = await RuntimeService.Start(requestID, runtime.version);
      else if (action === "stop") receipt = await RuntimeService.Stop(requestID, runtime.version);
      else if (action === "restart")
        receipt = await RuntimeService.Restart(requestID, runtime.version);
      else receipt = await RuntimeService.RetryCleanup(requestID, runtime.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };

  return (
    <>
      <PageHeading title={t("server.title")} description={t("server.description")} />
      <SettingsSection title={t("server.runtime")}>
        <SettingRow title={t("server.status")} description={t("server.statusDescription")}>
          <RowActions>
            <AnimatedBadge status={presentationStatus(runtime.presentation)} size="sm">
              {runtime.presentation.label}
            </AnimatedBadge>
            {runtime.capabilities.canStartRuntime ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={actionState === "loading"}
                onClick={() => void invoke("start")}
              >
                {t("common.start")}
              </Button>
            ) : null}
            {runtime.capabilities.canStopRuntime || runtime.capabilities.canRetryCleanupRuntime ? (
              <Button
                variant="secondary"
                size="sm"
                disabled={actionState === "loading"}
                onClick={() =>
                  void invoke(runtime.capabilities.canRetryCleanupRuntime ? "retry" : "stop")
                }
              >
                {runtime.capabilities.canRetryCleanupRuntime
                  ? t("common.retryCleanup")
                  : t("common.stop")}
              </Button>
            ) : null}
            {runtime.capabilities.canRestartRuntime ? (
              <StatefulButton
                variant="secondary"
                size="sm"
                state={actionState}
                loadingText={t("server.restarting")}
                successText={t("server.restarted")}
                icon={<RotateCcw className="size-3.5" />}
                onClick={() => void invoke("restart")}
              >
                {t("common.restart")}
              </StatefulButton>
            ) : null}
          </RowActions>
        </SettingRow>
      </SettingsSection>

      {actionError ? (
        <ActionNotice component={t("dock.server")} message={actionError} />
      ) : runtime.phase === RuntimePhase.RuntimeFailed ? (
        <ActionNotice
          component={t("dock.server")}
          message={t("server.failedMessage")}
          actionLabel={
            runtime.capabilities.canRetryCleanupRuntime ? t("common.retryCleanup") : undefined
          }
          onAction={
            runtime.capabilities.canRetryCleanupRuntime ? () => void invoke("retry") : undefined
          }
        />
      ) : null}

      {draft.runtime ? (
        <RuntimeConfigWorkspace
          settings={draft.runtime}
          disabled={draft.phase === "preparing" || draft.phase === "saving"}
          updateSetting={draft.updateRuntime}
          showToast={showToast}
        />
      ) : (
        <SettingsLoading label={t("server.loadingSettings")} />
      )}
    </>
  );
}
