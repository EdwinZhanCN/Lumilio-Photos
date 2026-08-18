import i18n from "i18next";
import { ErrorCode } from "../../../bindings/desktop/internal/control/dto/models.js";

const messages: Record<ErrorCode, () => string> = {
  [ErrorCode.ErrorInvalidArgument]: () => i18n.t("errors.invalidArgument", "The request is not valid."),
  [ErrorCode.ErrorStaleVersion]: () => i18n.t("errors.staleVersion", "This view is out of date. Reloading the latest state is required."),
  [ErrorCode.ErrorStaleConfig]: () => i18n.t("errors.staleConfig", "The runtime configuration changed. Review it again before saving."),
  [ErrorCode.ErrorControllerBusy]: () => i18n.t("errors.controllerBusy", "The Desktop controller is busy. Try again shortly."),
  [ErrorCode.ErrorOperationConflict]: () => i18n.t("errors.operationConflict", "Another operation is already using this control surface."),
  [ErrorCode.ErrorOperationNotCancellable]: () => i18n.t("errors.operationNotCancellable", "This operation has reached a non-cancellable point."),
  [ErrorCode.ErrorRuntimeNotConfigured]: () => i18n.t("errors.runtimeNotConfigured", "Complete the runtime configuration first."),
  [ErrorCode.ErrorRuntimeNotReady]: () => i18n.t("errors.runtimeNotReady", "The runtime is not ready yet."),
  [ErrorCode.ErrorRepositoryControlUnavailable]: () => i18n.t("errors.repositoryControlUnavailable", "Storage control is available only after the runtime is ready."),
  [ErrorCode.ErrorStorageLocationOffline]: () => i18n.t("errors.storageLocationOffline", "This Storage Location is offline or no longer authorized."),
  [ErrorCode.ErrorStopTimeout]: () => i18n.t("errors.stopTimeout", "The process did not stop within the safety budget; cleanup is required."),
  [ErrorCode.ErrorReadinessTimeout]: () => i18n.t("errors.readinessTimeout", "The process did not become ready within the safety budget."),
  [ErrorCode.ErrorSignatureInvalid]: () => i18n.t("errors.signatureInvalid", "The downloaded artifact failed signature verification."),
  [ErrorCode.ErrorShutdownInProgress]: () => i18n.t("errors.shutdownInProgress", "Desktop is shutting down or recovering."),
  [ErrorCode.ErrorLumenNotInstalled]: () => i18n.t("errors.lumenNotInstalled", "Install Lumen Hub before starting it."),
  [ErrorCode.ErrorLumenOwnerBusy]: () => i18n.t("errors.lumenOwnerBusy", "Another Desktop instance owns Lumen Hub."),
  [ErrorCode.ErrorRecoveryRequired]: () => i18n.t("errors.recoveryRequired", "Desktop needs recovery before this operation can continue."),
  [ErrorCode.$zero]: () => i18n.t("errors.unknown", "The Desktop control plane returned an unknown error."),
};

export function errorMessage(reason: unknown): string {
  if (typeof reason === "object" && reason !== null && "code" in reason) {
    const code = (reason as { code?: ErrorCode }).code ?? ErrorCode.$zero;
    return (messages[code] ?? messages[ErrorCode.$zero])();
  }
  return reason instanceof Error ? reason.message : messages[ErrorCode.$zero]();
}
