import { FolderOpen, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { LumenService } from "../../../bindings/desktop/internal/control/index.js";
import {
  LumenInstallPhase,
  LumenControlPhase,
  LumenProcessPhase,
  type DesktopSnapshot,
  type LumenLogEntry,
  type LumenSnapshot,
  type OperationReceipt,
} from "../../../bindings/desktop/internal/control/dto/models.js";
import { AnimatedBadge, type AnimatedBadgeStatus } from "@/components/motion/animated-badge";
import { type ToastInput } from "@/components/motion/animated-toast-stack";
import { Button, StatefulButton } from "@/components/motion/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/motion/select";
import {
  InlineNotice,
  PageHeading,
  SettingRow,
  SettingsSection,
} from "@/components/settings/setting-layout";
import { errorMessage } from "@/lib/desktop/errors";
import { ActionNotice, RowActions, presentationStatus, useTrackedOperation } from "../shared";

export function LumenPanel({
  snapshot,
  showToast,
}: {
  snapshot: DesktopSnapshot;
  showToast: (input: ToastInput) => string;
}) {
  const { t } = useTranslation();
  const lumen = snapshot.lumen;
  const [inputError, setInputError] = useState<string | null>(null);
  const operation = useTrackedOperation(snapshot.operations, {
    onFailed: (message) =>
      showToast({ title: t("lumen.actionFailed"), description: message, status: "error" }),
  });
  const state = operation.state;
  const actionError = operation.error ?? inputError;
  const [selectedProfile, setSelectedProfile] = useState(
    lumen.profile || lumen.availableProfiles?.[0] || "",
  );
  const [selectedPreset, setSelectedPreset] = useState(
    lumen.preset || lumen.availablePresets?.[0] || "",
  );
  const [selectedCacheDir, setSelectedCacheDir] = useState(lumen.cacheDir || "");
  const [logLevel, setLogLevel] = useState("INFO");
  const [logs, setLogs] = useState<LumenLogEntry[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);
  const releaseReady = lumen.installerAvailable && lumen.processAvailable;

  useEffect(() => {
    setSelectedProfile(lumen.profile || lumen.availableProfiles?.[0] || "");
    setSelectedPreset(lumen.preset || lumen.availablePresets?.[0] || "");
    setSelectedCacheDir(lumen.cacheDir || "");
  }, [
    lumen.cacheDir,
    lumen.preset,
    lumen.profile,
    lumen.availablePresets,
    lumen.availableProfiles,
  ]);

  const loadLogs = useCallback(async () => {
    if (lumen.processPhase !== LumenProcessPhase.LumenRunning || !lumen.control.connected) {
      setLogs([]);
      setLogsError(null);
      return;
    }
    setLogsLoading(true);
    try {
      setLogs((await LumenService.GetLogs(200, logLevel)) ?? []);
      setLogsError(null);
    } catch (reason: unknown) {
      setLogsError(errorMessage(reason));
    } finally {
      setLogsLoading(false);
    }
  }, [logLevel, lumen.control.connected, lumen.processPhase]);

  useEffect(() => {
    if (lumen.processPhase !== LumenProcessPhase.LumenRunning || !lumen.control.connected) {
      setLogs([]);
      setLogsError(null);
      return;
    }
    void loadLogs();
    const timer = window.setInterval(() => void loadLogs(), 5000);
    return () => window.clearInterval(timer);
  }, [loadLogs, lumen.control.connected, lumen.processPhase]);

  const invoke = async (action: "install" | "start" | "stop" | "restart" | "retry") => {
    operation.begin();
    setInputError(null);
    const requestID = `lumen-${crypto.randomUUID()}`;
    try {
      let receipt: OperationReceipt;
      if (action === "install")
        receipt = await LumenService.Install(
          requestID,
          lumen.version,
          selectedProfile,
          selectedPreset,
          selectedCacheDir,
        );
      else if (action === "start") receipt = await LumenService.Start(requestID, lumen.version);
      else if (action === "stop") receipt = await LumenService.Stop(requestID, lumen.version);
      else if (action === "restart") receipt = await LumenService.Restart(requestID, lumen.version);
      else receipt = await LumenService.RetryCleanup(requestID, lumen.version);
      operation.track(receipt);
    } catch (reason: unknown) {
      operation.reject(reason);
    }
  };

  const installed = lumen.installPhase === LumenInstallPhase.LumenInstalled;
  const setupChanged = selectedPreset !== lumen.preset || selectedCacheDir !== lumen.cacheDir;
  const canSubmitSetup =
    lumen.installerAvailable &&
    Boolean(selectedProfile) &&
    Boolean(selectedPreset) &&
    Boolean(selectedCacheDir) &&
    state !== "loading" &&
    (!installed || setupChanged);
  const canChooseIntent = state !== "loading";
  const canChooseProfile = !installed && canChooseIntent;

  const chooseCacheDirectory = async () => {
    try {
      const path = await LumenService.PickCacheDirectory(
        t("lumen.chooseCacheDirectory", "Choose the Lumen model cache directory"),
      );
      if (path) {
        setSelectedCacheDir(path);
        setInputError(null);
      }
    } catch (reason: unknown) {
      const message = errorMessage(reason);
      setInputError(message);
      showToast({
        title: t("lumen.cachePickFailed", "Cache directory could not be selected"),
        description: message,
        status: "error",
      });
    }
  };

  const presetLabel = (preset: string) => {
    if (preset === "minimal") return t("lumen.presetNameMinimal", "Minimal");
    if (preset === "brave") return t("lumen.presetNameBrave", "Brave");
    if (preset === "basic") return t("lumen.presetNameBasic", "Basic");
    return preset;
  };

  const backendLabel = (profile: string) => {
    if (profile.endsWith("-metal")) return t("lumen.backendMetal", "Metal");
    if (profile.endsWith("-gpu")) return t("lumen.backendGPU", "GPU (WGPU)");
    return t("lumen.backendCPU", "CPU");
  };

  return (
    <>
      <PageHeading title={t("dock.lumen")} description={t("lumen.description")} />
      <SettingsSection title={t("lumen.availability")}>
        <SettingRow
          title="Lumen Hub"
          description={releaseReady ? t("lumen.releaseReady") : t("lumen.notIncluded")}
        >
          <AnimatedBadge
            status={releaseReady ? presentationStatus(lumen.presentation) : "neutral"}
            size="sm"
          >
            {releaseReady ? lumen.presentation.label : t("onboarding.unavailable")}
          </AnimatedBadge>
        </SettingRow>
      </SettingsSection>

      {releaseReady ? (
        <SettingsSection title={t("lumen.service")}>
          <SettingRow
            title={t("lumen.preset", "Preset")}
            description={t(
              "lumen.presetDescription",
              "Choose a runtime preset exposed by the pinned Lumen Hub release.",
            )}
          >
            <Select
              value={selectedPreset}
              onValueChange={setSelectedPreset}
              disabled={!canChooseIntent}
              className="w-80 max-w-full"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(lumen.availablePresets ?? []).map((preset) => (
                  <SelectItem key={preset} value={preset}>
                    {presetLabel(preset)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.backend", "Backend")}
            description={t(
              "lumen.backendDescription",
              "Backend determines which platform-specific Lumen Hub package is downloaded.",
            )}
          >
            <Select
              value={selectedProfile}
              onValueChange={setSelectedProfile}
              disabled={!canChooseProfile}
              className="w-80 max-w-full"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(lumen.availableProfiles ?? []).map((profile) => (
                  <SelectItem key={profile} value={profile}>
                    {backendLabel(profile)} · {profile}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </SettingRow>
          <SettingRow
            title={t("lumen.cacheDirectory", "Model cache")}
            description={
              selectedCacheDir ||
              t("lumen.cacheDirectoryDescription", "Choose where Lumen stores downloaded models.")
            }
          >
            <Button
              variant="secondary"
              size="sm"
              disabled={!canChooseIntent}
              onClick={() => void chooseCacheDirectory()}
            >
              <FolderOpen className="size-3.5" /> {t("common.choose")}
            </Button>
          </SettingRow>
          <SettingRow
            title={installed ? t("lumen.reconfigure", "Reconfigure") : t("lumen.installation")}
            description={
              installed
                ? lumen.processPhase === LumenProcessPhase.LumenRunning
                  ? t(
                      "lumen.reconfigureRunning",
                      "The new preset and cache are validated first, then applied with a controlled restart. The previous setup is restored if startup fails.",
                    )
                  : t(
                      "lumen.reconfigureStopped",
                      "The new preset and cache are validated before they replace the current setup.",
                    )
                : selectedProfile
                  ? `${t("lumen.profile")}: ${selectedProfile}`
                  : t("lumen.noProfile")
            }
          >
            <StatefulButton
              variant="secondary"
              size="sm"
              state={state}
              disabled={!canSubmitSetup}
              loadingText={
                installed
                  ? t("lumen.applyingConfiguration", "Applying configuration")
                  : t("lumen.installing")
              }
              successText={
                installed
                  ? t("lumen.configurationApplied", "Configuration applied")
                  : t("lumen.installed")
              }
              onClick={() => void invoke("install")}
            >
              {installed
                ? t("lumen.applyConfiguration", "Apply configuration")
                : t("lumen.install")}
            </StatefulButton>
          </SettingRow>
          <SettingRow
            title={t("lumen.processStatus")}
            description={`${t("lumen.desiredState")}: ${lumen.desiredState || "disabled"}.`}
          >
            <RowActions>
              <AnimatedBadge status={presentationStatus(lumen.presentation)} size="sm">
                {lumen.presentation.label}
              </AnimatedBadge>
              {lumen.capabilities.canStartLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("start")}
                >
                  {t("common.start")}
                </Button>
              ) : null}
              {lumen.capabilities.canStopLumen || lumen.capabilities.canRetryCleanupLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() =>
                    void invoke(lumen.capabilities.canRetryCleanupLumen ? "retry" : "stop")
                  }
                >
                  {lumen.capabilities.canRetryCleanupLumen
                    ? t("common.retryCleanup")
                    : t("common.stop")}
                </Button>
              ) : null}
              {lumen.capabilities.canRestartLumen ? (
                <Button
                  variant="secondary"
                  size="sm"
                  disabled={state === "loading"}
                  onClick={() => void invoke("restart")}
                >
                  {t("common.restart")}
                </Button>
              ) : null}
            </RowActions>
          </SettingRow>
        </SettingsSection>
      ) : null}

      {releaseReady && lumen.installPhase === LumenInstallPhase.LumenInstalled ? (
        <LumenControlPanel lumen={lumen} />
      ) : null}

      {releaseReady && lumen.installPhase === LumenInstallPhase.LumenInstalled ? (
        <SettingsSection
          title={t("lumen.logs", "Control logs")}
          description={t(
            "lumen.logsDescription",
            "Structured logs from Lumen Control. The view refreshes every five seconds while Hub is running.",
          )}
        >
          <div className="lumen-log-toolbar">
            <Select
              value={logLevel}
              onValueChange={setLogLevel}
              disabled={!lumen.control.connected}
              className="compact-select"
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(["TRACE", "DEBUG", "INFO", "WARN", "ERROR"] as const).map((level) => (
                  <SelectItem key={level} value={level}>
                    {level}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              variant="secondary"
              size="sm"
              disabled={!lumen.control.connected || logsLoading}
              onClick={() => void loadLogs()}
            >
              <RefreshCw className={logsLoading ? "size-3.5 animate-spin" : "size-3.5"} />
              {t("common.refresh", "Refresh")}
            </Button>
          </div>
          {logsError ? (
            <InlineNotice tone="danger" title={t("lumen.logsUnavailable", "Logs unavailable")}>
              {logsError}
            </InlineNotice>
          ) : null}
          <LumenLogViewer logs={logs} connected={lumen.control.connected} loading={logsLoading} />
        </SettingsSection>
      ) : null}

      {actionError ? <ActionNotice component={t("dock.lumen")} message={actionError} /> : null}
    </>
  );
}

const lumenPhaseOrder = [
  LumenControlPhase.LumenControlStarting,
  LumenControlPhase.LumenControlDownloading,
  LumenControlPhase.LumenControlLoading,
  LumenControlPhase.LumenControlWarmup,
  LumenControlPhase.LumenControlReady,
] as const;

function LumenControlPanel({ lumen }: { lumen: LumenSnapshot }) {
  const { t } = useTranslation();
  const control = lumen.control;
  const currentIndex = lumenPhaseOrder.indexOf(control.phase as (typeof lumenPhaseOrder)[number]);
  const downloadPercent = control.download?.bytesTotal
    ? Math.min(100, Math.max(0, (control.download.bytesDone / control.download.bytesTotal) * 100))
    : null;

  return (
    <SettingsSection
      title={t("lumen.controlStatus", "Control status")}
      description={t(
        "lumen.controlDescription",
        "Live lifecycle and model state reported by lumen.control.v1.",
      )}
    >
      <div className="lumen-control-summary">
        <div>
          <span className="lumen-control-kicker">{t("lumen.inference", "Inference")}</span>
          <strong>
            {control.inferenceReady
              ? t("lumen.ready", "Ready")
              : control.connected
                ? t("lumen.preparing", "Preparing")
                : t("lumen.disconnected", "Disconnected")}
          </strong>
        </div>
        <AnimatedBadge status={controlPhaseStatus(control.phase)} size="sm">
          {controlPhaseLabel(control.phase, t)}
        </AnimatedBadge>
        <dl className="lumen-control-meta">
          <div>
            <dt>{t("lumen.version", "Version")}</dt>
            <dd>{control.version || "—"}</dd>
          </div>
          <div>
            <dt>{t("lumen.backend", "Backend")}</dt>
            <dd>{control.backend || "—"}</dd>
          </div>
          <div>
            <dt>{t("lumen.sequence", "Sequence")}</dt>
            <dd>{control.sequence || "—"}</dd>
          </div>
        </dl>
      </div>

      {control.connected ? (
        <ol
          className="lumen-phase-track"
          aria-label={t("lumen.lifecycle", "Lumen startup lifecycle")}
        >
          {lumenPhaseOrder.map((phase, index) => (
            <li
              key={phase}
              className={
                index < currentIndex ? "complete" : index === currentIndex ? "current" : undefined
              }
              aria-current={index === currentIndex ? "step" : undefined}
            >
              <span aria-hidden />
              <small>{controlPhaseLabel(phase, t)}</small>
            </li>
          ))}
        </ol>
      ) : (
        <InlineNotice title={t("lumen.controlWaiting", "Waiting for Control")}>
          {t(
            "lumen.controlWaitingDescription",
            "Start Lumen Hub to connect to its local control plane.",
          )}
        </InlineNotice>
      )}

      {control.download ? (
        <div
          className="lumen-download"
          aria-label={t("lumen.downloadProgress", "Model download progress")}
        >
          <div className="lumen-download-heading">
            <div>
              <strong>{control.download.model || t("lumen.model", "Model")}</strong>
              <span>
                {control.download.file || t("lumen.preparingDownload", "Preparing download")}
              </span>
            </div>
            <span className="tabular-value">
              {downloadPercent === null
                ? t("lumen.downloading", "Downloading")
                : `${downloadPercent.toFixed(1)}%`}
            </span>
          </div>
          <progress
            value={control.download.bytesDone}
            max={control.download.bytesTotal || undefined}
          />
          <div className="lumen-download-meta">
            <span>
              {formatBytes(control.download.bytesDone)}
              {control.download.bytesTotal ? ` / ${formatBytes(control.download.bytesTotal)}` : ""}
            </span>
            <span>
              {t("lumen.filesProgress", "Files {{done}} / {{total}}", {
                done: control.download.filesDone,
                total: control.download.filesTotal,
              })}
            </span>
          </div>
        </div>
      ) : null}

      {control.error ? (
        <InlineNotice tone="danger" title={t("lumen.controlFailed", "Lumen startup failed")}>
          {control.error.message}
        </InlineNotice>
      ) : null}

      <div className="lumen-services">
        <div className="lumen-subheading">
          <h3>{t("lumen.services", "AI services")}</h3>
          <span>
            {t("lumen.servicesReported", "{{count}} reported", {
              count: control.services?.length ?? 0,
            })}
          </span>
        </div>
        {control.services?.length ? (
          control.services.map((service) => (
            <div className="lumen-service-row" key={service.service}>
              <div>
                <strong>{serviceDisplayName(service.service)}</strong>
                {service.error ? <span>{service.error.message}</span> : null}
              </div>
              <AnimatedBadge status={controlPhaseStatus(service.phase)} size="sm">
                {controlPhaseLabel(service.phase, t)}
              </AnimatedBadge>
            </div>
          ))
        ) : (
          <p className="lumen-empty-copy">
            {control.connected
              ? t("lumen.servicesPending", "Service states will appear after model construction.")
              : t("lumen.servicesOffline", "No service state is available while Hub is stopped.")}
          </p>
        )}
      </div>
    </SettingsSection>
  );
}

function LumenLogViewer({
  logs,
  connected,
  loading,
}: {
  logs: LumenLogEntry[];
  connected: boolean;
  loading: boolean;
}) {
  const { t } = useTranslation();
  if (!connected)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsOffline", "Start Lumen Hub to read Control logs.")}
      </p>
    );
  if (!logs.length && loading)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsLoading", "Reading Control logs…")}
      </p>
    );
  if (!logs.length)
    return (
      <p className="lumen-empty-copy lumen-log-empty">
        {t("lumen.logsEmpty", "No log entries match this level.")}
      </p>
    );
  return (
    <div className="lumen-log-view" role="log" aria-label={t("lumen.logs", "Control logs")}>
      {logs.map((entry, index) => (
        <div
          className="lumen-log-line"
          key={`${entry.timeUnixMS}-${index}`}
          data-level={entry.level}
        >
          <time>{formatLogTime(entry.timeUnixMS)}</time>
          <span className="lumen-log-level">{entry.level}</span>
          <span className="lumen-log-target">{entry.target}</span>
          <span className="lumen-log-message">
            {entry.message}
            {formatLogFields(entry.fields)}
          </span>
        </div>
      ))}
    </div>
  );
}

function controlPhaseStatus(phase: LumenControlPhase): AnimatedBadgeStatus {
  if (phase === LumenControlPhase.LumenControlReady) return "success";
  if (phase === LumenControlPhase.LumenControlFailed) return "danger";
  if (
    phase === LumenControlPhase.LumenControlUnspecified ||
    phase === LumenControlPhase.LumenControlStopping
  )
    return "neutral";
  return "warning";
}

function controlPhaseLabel(phase: LumenControlPhase, t: ReturnType<typeof useTranslation>["t"]) {
  if (phase === LumenControlPhase.LumenControlStarting) return t("lumen.phaseStarting", "Starting");
  if (phase === LumenControlPhase.LumenControlDownloading)
    return t("lumen.phaseDownloading", "Downloading");
  if (phase === LumenControlPhase.LumenControlLoading) return t("lumen.phaseLoading", "Loading");
  if (phase === LumenControlPhase.LumenControlWarmup) return t("lumen.phaseWarmup", "Warmup");
  if (phase === LumenControlPhase.LumenControlReady) return t("lumen.phaseReady", "Ready");
  if (phase === LumenControlPhase.LumenControlFailed) return t("lumen.phaseFailed", "Failed");
  if (phase === LumenControlPhase.LumenControlStopping) return t("lumen.phaseStopping", "Stopping");
  return t("lumen.phaseUnavailable", "Unavailable");
}

function serviceDisplayName(service: string) {
  const names: Record<string, string> = {
    siglip: "SigLIP",
    face: "InsightFace",
    insightface: "InsightFace",
    ocr: "PP-OCR",
    ppocr: "PP-OCR",
    bioclip: "BioCLIP",
  };
  return names[service] || service;
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  return `${(bytes / 1024 ** index).toFixed(index > 1 ? 1 : 0)} ${units[index]}`;
}

function formatLogTime(unixMS: number) {
  if (!unixMS) return "--:--:--";
  return new Date(unixMS).toLocaleTimeString([], {
    hour12: false,
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function formatLogFields(fields: LumenLogEntry["fields"]) {
  const entries = Object.entries(fields ?? {});
  return entries.length ? ` · ${entries.map(([key, value]) => `${key}=${value}`).join(" ")}` : "";
}
