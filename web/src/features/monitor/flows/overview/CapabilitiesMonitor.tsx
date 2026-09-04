import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  CircleDashed,
  Cpu,
  Loader2,
  Network,
  RefreshCcw,
  Server,
  Sparkles,
  XCircle,
} from "lucide-react";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useLumenRuntime,
  type LumenBackendStatus,
  type LumenNodeRuntime,
} from "../../api/useLumenRuntime";

type RuntimeState = "disabled" | "starting" | "healthy" | "degraded";
type Translate = ReturnType<typeof useI18n>["t"];

function availabilityBadgeClass(available: boolean) {
  return available ? "badge badge-success" : "badge badge-error badge-outline";
}

function enabledBadgeClass(enabled: boolean) {
  return enabled ? "badge badge-info badge-outline" : "badge badge-ghost";
}

function stateBadgeClass(state?: string) {
  switch (state) {
    case "healthy":
    case "compatible":
    case "ready":
      return "badge badge-success badge-outline";
    case "degraded":
    case "incompatible":
    case "unavailable":
      return "badge badge-error badge-outline";
    case "starting":
    case "pending":
    case "connecting":
      return "badge badge-warning badge-outline";
    default:
      return "badge badge-ghost";
  }
}

function stateLabel(t: Translate, state?: string) {
  switch (state) {
    case "disabled":
      return t("monitor.capabilities.states.disabled", "disabled");
    case "starting":
      return t("monitor.capabilities.states.starting", "starting");
    case "healthy":
      return t("monitor.capabilities.states.healthy", "healthy");
    case "degraded":
      return t("monitor.capabilities.states.degraded", "degraded");
    case "connecting":
      return t("monitor.capabilities.states.connecting", "connecting");
    case "ready":
      return t("monitor.capabilities.states.ready", "ready");
    case "unavailable":
      return t("monitor.capabilities.states.unavailable", "unavailable");
    case "pending":
      return t("monitor.capabilities.states.pending", "pending");
    case "compatible":
      return t("monitor.capabilities.states.compatible", "compatible");
    case "incompatible":
      return t("monitor.capabilities.states.incompatible", "incompatible");
    case "active":
      return t("monitor.capabilities.states.active", "active");
    default:
      return t("monitor.capabilities.states.unknown", "unknown");
  }
}

function formatTimestamp(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "—";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "medium",
  }).format(date);
}

function errorCodeLabel(t: Translate, code?: string) {
  switch (code) {
    case "query_timed_out":
      return t("monitor.capabilities.errors.queryTimedOut", "Discovery scan timed out");
    case "query_failed":
      return t("monitor.capabilities.errors.queryFailed", "Discovery scan failed");
    case "socket_open_failed":
      return t("monitor.capabilities.errors.socketOpenFailed", "Discovery socket could not open");
    case "socket_send_failed":
      return t("monitor.capabilities.errors.socketSendFailed", "Discovery query could not be sent");
    case "socket_read_failed":
      return t(
        "monitor.capabilities.errors.socketReadFailed",
        "Discovery replies could not be read",
      );
    case "resolve_failed":
      return t(
        "monitor.capabilities.errors.resolveFailed",
        "A matching service had no usable address",
      );
    case "watch_start_failed":
      return t("monitor.capabilities.errors.watchStartFailed", "Discovery backend could not start");
    case "watch_closed":
      return t("monitor.capabilities.errors.watchClosed", "Discovery backend stopped unexpectedly");
    case "transport_unavailable":
      return t("monitor.capabilities.errors.transportUnavailable", "Transport unavailable");
    case "capability_rpc_unimplemented":
      return t(
        "monitor.capabilities.errors.capabilityUnimplemented",
        "Capability exchange is not implemented",
      );
    case "protocol_incompatible":
      return t(
        "monitor.capabilities.errors.protocolIncompatible",
        "Protocol version is incompatible",
      );
    case "incompatible":
      return t("monitor.capabilities.errors.incompatible", "Node is incompatible");
    default:
      return code ? code.replaceAll("_", " ") : "—";
  }
}

function CapabilityRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="text-base-content/60">{label}</span>
      <span className="font-medium text-right">{value}</span>
    </div>
  );
}

function DiscoveryHealth({ state }: { state: RuntimeState }) {
  const { t } = useI18n();
  const content = {
    disabled: {
      icon: XCircle,
      title: t("monitor.capabilities.discovery.disabled", "Lumen discovery is disabled"),
      detail: t(
        "monitor.capabilities.discovery.disabledDetail",
        "Media management remains available, but external ML tasks cannot run.",
      ),
      className: "border-base-300 bg-base-200/50",
    },
    starting: {
      icon: CircleDashed,
      title: t("monitor.capabilities.discovery.starting", "Lumen discovery is starting"),
      detail: t(
        "monitor.capabilities.discovery.startingDetail",
        "The first bounded discovery scan has not completed yet.",
      ),
      className: "border-warning/40 bg-warning/10",
    },
    healthy: {
      icon: CheckCircle2,
      title: t("monitor.capabilities.discovery.healthy", "Lumen discovery is healthy"),
      detail: t(
        "monitor.capabilities.discovery.healthyDetail",
        "Scheduled discovery is running independently of this page.",
      ),
      className: "border-success/40 bg-success/10",
    },
    degraded: {
      icon: AlertTriangle,
      title: t("monitor.capabilities.discovery.degraded", "Lumen discovery is degraded"),
      detail: t(
        "monitor.capabilities.discovery.degradedDetail",
        "Previous observations are preserved while the backend retries automatically.",
      ),
      className: "border-error/40 bg-error/10",
    },
  }[state];
  const Icon = content.icon;
  return (
    <div
      className={`flex items-start gap-3 rounded-lg border px-4 py-3 ${content.className}`}
      role="status"
    >
      <Icon className="mt-0.5 size-5 shrink-0" aria-hidden="true" />
      <div>
        <div className="font-semibold">{content.title}</div>
        <p className="mt-0.5 text-sm text-base-content/70">{content.detail}</p>
      </div>
    </div>
  );
}

function BackendTable({ backends }: { backends: LumenBackendStatus[] }) {
  const { t } = useI18n();
  return (
    <section className="overflow-hidden rounded-lg border border-base-300 bg-base-100">
      <div className="border-b border-base-300 px-4 py-3">
        <h2 className="font-semibold">
          {t("monitor.capabilities.discoveryBackends", "Discovery backends")}
        </h2>
      </div>
      <div className="overflow-x-auto">
        <table className="table table-sm">
          <thead>
            <tr>
              <th>{t("monitor.capabilities.source", "Source")}</th>
              <th>{t("monitor.capabilities.state", "State")}</th>
              <th>{t("monitor.capabilities.lastSuccess", "Last successful scan")}</th>
              <th>{t("monitor.capabilities.scanResults", "Matched / rejected")}</th>
              <th>{t("monitor.capabilities.diagnostic", "Diagnostic")}</th>
            </tr>
          </thead>
          <tbody>
            {backends.map((backend) => (
              <tr key={backend.source ?? "unknown"}>
                <td className="font-mono text-xs">{backend.source ?? "—"}</td>
                <td>
                  <span className={stateBadgeClass(backend.state)}>
                    {stateLabel(t, backend.state)}
                  </span>
                </td>
                <td className="whitespace-nowrap text-xs">
                  {formatTimestamp(backend.last_scan_succeeded_at)}
                </td>
                <td className="font-mono text-xs">
                  {backend.matched_count ?? 0} / {backend.rejected_count ?? 0}
                </td>
                <td className="text-xs">
                  {backend.last_error_code ? (
                    <span>
                      {errorCodeLabel(t, backend.last_error_code)}
                      {(backend.consecutive_failures ?? 0) > 0 &&
                        ` · ${t("monitor.capabilities.consecutiveFailures", {
                          count: backend.consecutive_failures,
                          defaultValue: "{{count}} consecutive failures",
                        })}`}
                    </span>
                  ) : (
                    "—"
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function nodeVerdict(node: LumenNodeRuntime) {
  if (node.transport === "unavailable") return "unavailable";
  if (node.transport !== "ready") return "connecting";
  if (node.compatibility === "incompatible") return "incompatible";
  if (node.compatibility !== "compatible") return "pending";
  return "active";
}

function NodeTable({
  nodes,
  discoveryState,
}: {
  nodes: LumenNodeRuntime[];
  discoveryState: RuntimeState;
}) {
  const { t } = useI18n();
  if (nodes.length === 0) {
    return (
      <section className="rounded-lg border border-base-300 bg-base-100 px-5 py-8 text-center">
        <Server className="mx-auto size-7 text-base-content/40" aria-hidden="true" />
        <h2 className="mt-3 font-semibold">
          {discoveryState === "healthy"
            ? t("monitor.capabilities.noNodes", "No validated Lumen Hubs are advertised")
            : t(
                "monitor.capabilities.noCurrentNodes",
                "No validated Lumen Hubs are currently visible",
              )}
        </h2>
        <p className="mx-auto mt-1 max-w-xl text-sm text-base-content/60">
          {t(
            "monitor.capabilities.noNodesDetail",
            "A Hub that appears later will be discovered and validated without restarting Photos or opening Monitor.",
          )}
        </p>
      </section>
    );
  }

  return (
    <section className="overflow-hidden rounded-lg border border-base-300 bg-base-100">
      <div className="border-b border-base-300 px-4 py-3">
        <h2 className="font-semibold">
          {t("monitor.capabilities.validatedNodes", "Validated nodes")}
        </h2>
      </div>
      <div className="overflow-x-auto">
        <table className="table table-sm">
          <thead>
            <tr>
              <th>{t("monitor.capabilities.node", "Node")}</th>
              <th>{t("monitor.capabilities.source", "Source")}</th>
              <th>{t("monitor.capabilities.state", "State")}</th>
              <th>{t("monitor.capabilities.transport", "Transport")}</th>
              <th>{t("monitor.capabilities.compatibility", "Compatibility")}</th>
              <th>{t("monitor.capabilities.tasks", "Advertised tasks")}</th>
              <th>{t("monitor.capabilities.lastObserved", "Last observed")}</th>
            </tr>
          </thead>
          <tbody>
            {nodes.map((node) => {
              const verdict = nodeVerdict(node);
              return (
                <tr key={node.id ?? node.endpoint}>
                  <td>
                    <div className="font-medium">{node.id ?? "—"}</div>
                    <div className="font-mono text-xs text-base-content/60">
                      {node.endpoint ?? "—"}
                    </div>
                    {(node.version || node.runtime) && (
                      <div className="mt-1 text-xs text-base-content/50">
                        {[node.version, node.runtime].filter(Boolean).join(" · ")}
                      </div>
                    )}
                  </td>
                  <td className="font-mono text-xs">{node.sources?.join(", ") || "—"}</td>
                  <td>
                    <span className={stateBadgeClass(verdict)}>{stateLabel(t, verdict)}</span>
                  </td>
                  <td>
                    <span className={stateBadgeClass(node.transport)}>
                      {stateLabel(t, node.transport)}
                    </span>
                  </td>
                  <td>
                    <div className="flex flex-col items-start gap-1">
                      <span className={stateBadgeClass(node.compatibility)}>
                        {stateLabel(t, node.compatibility)}
                      </span>
                      {node.error_code && (
                        <span className="max-w-52 text-xs text-base-content/60">
                          {errorCodeLabel(t, node.error_code)}
                        </span>
                      )}
                    </div>
                  </td>
                  <td>
                    {node.tasks && node.tasks.length > 0 ? (
                      <ul className="space-y-1 font-mono text-xs">
                        {node.tasks.map((task) => (
                          <li key={`${task.service}:${task.task}`}>
                            {task.service || "service"} / {task.task || "task"}
                          </li>
                        ))}
                      </ul>
                    ) : (
                      <span className="text-xs text-base-content/50">
                        {node.compatibility === "pending"
                          ? t(
                              "monitor.capabilities.capabilityPending",
                              "Capability exchange pending",
                            )
                          : "—"}
                      </span>
                    )}
                  </td>
                  <td className="whitespace-nowrap text-xs">
                    {formatTimestamp(node.last_observed_at)}
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export function CapabilitiesMonitor() {
  const { t } = useI18n();
  const capabilityQuery = useCapabilities(5000);
  const runtimeQuery = useLumenRuntime(5000);
  const capabilities = capabilityQuery.capabilities;
  const runtime = runtimeQuery.data;

  if ((capabilityQuery.isLoading && !capabilities) || (runtimeQuery.isLoading && !runtime)) {
    return (
      <div className="rounded-lg bg-base-100 p-6 text-center shadow-sm">
        <Loader2 className="mx-auto size-8 animate-spin text-primary" />
        <p className="mt-3 text-sm text-base-content/60">{t("common.loading")}</p>
      </div>
    );
  }

  if (capabilityQuery.isError || runtimeQuery.isError || !capabilities || !runtime) {
    return (
      <div className="rounded-lg border border-warning/40 bg-warning/10 p-6 text-center">
        <div className="text-sm font-medium">{t("settings.serverSettings.capabilitiesError")}</div>
      </div>
    );
  }

  const semanticEnabled = capabilities.ml.tasks.clipImageEmbed.enabled;
  const semanticAvailable =
    capabilities.ml.tasks.clipImageEmbed.available &&
    capabilities.ml.tasks.semanticTextEmbed.available;
  const mlCapabilities = [
    {
      key: "semantic",
      label: t("settings.aiSettings.taskNames.semantic", "Image Semantic Analysis"),
      enabled: semanticEnabled,
      available: semanticAvailable,
      tasks: ["semantic_image_embed", "semantic_text_embed"],
    },
    {
      key: "face",
      label: t("settings.aiSettings.taskNames.face", "Person Recognition"),
      enabled: capabilities.ml.tasks.faceDetectAndEmbed.enabled,
      available: capabilities.ml.tasks.faceDetectAndEmbed.available,
      tasks: ["face_recognition"],
    },
    {
      key: "ocr",
      label: t("settings.aiSettings.taskNames.ocr", "OCR Text Recognition"),
      enabled: capabilities.ml.tasks.ocr.enabled,
      available: capabilities.ml.tasks.ocr.available,
      tasks: ["ocr"],
    },
    {
      key: "bioclip",
      label: t("settings.serverSettings.taskNames.bioClipClassify", "BioCLIP Species Recognition"),
      enabled: capabilities.ml.tasks.bioClipClassify.enabled,
      available: capabilities.ml.tasks.bioClipClassify.available,
      tasks: ["bioclip_classify"],
    },
  ];
  const discoveryState = runtime.discovery_state ?? capabilities.ml.discoveryState;
  const isRefreshing = capabilityQuery.isFetching || runtimeQuery.isFetching;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end">
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => void Promise.all([capabilityQuery.refetch(), runtimeQuery.refetch()])}
          disabled={isRefreshing}
        >
          <RefreshCcw className={`size-4 ${isRefreshing ? "animate-spin" : ""}`} />
          {t("monitor.capabilities.refreshStatus", "Refresh status")}
        </button>
      </div>

      <DiscoveryHealth state={discoveryState} />

      <div className="grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-base-300 bg-base-300 xl:grid-cols-4">
        <div className="bg-base-100 p-4">
          <Network className="size-5 text-info" aria-hidden="true" />
          <div className="mt-3 text-2xl font-semibold">{runtime.counts?.discovered ?? 0}</div>
          <div className="text-sm text-base-content/60">
            {t("settings.serverSettings.discoveredNodes")}
          </div>
        </div>
        <div className="bg-base-100 p-4">
          <Server className="size-5 text-success" aria-hidden="true" />
          <div className="mt-3 text-2xl font-semibold">{runtime.counts?.active ?? 0}</div>
          <div className="text-sm text-base-content/60">
            {t("settings.serverSettings.activeNodes")}
          </div>
        </div>
        <div className="bg-base-100 p-4">
          <Cpu className="size-5 text-primary" aria-hidden="true" />
          <div className="mt-3 text-2xl font-semibold">
            {mlCapabilities.filter((capability) => capability.available).length} /{" "}
            {mlCapabilities.length}
          </div>
          <div className="text-sm text-base-content/60">
            {t("settings.serverSettings.taskAvailability")}
          </div>
        </div>
        <div className="bg-base-100 p-4">
          <Sparkles className="size-5 text-secondary" aria-hidden="true" />
          <div className="mt-3 text-lg font-semibold">
            {capabilities.llm.agentEnabled
              ? t("settings.serverSettings.enabled")
              : t("settings.serverSettings.disabled")}
          </div>
          <div className="text-sm text-base-content/60">
            {capabilities.llm.provider || t("settings.serverSettings.llmTitle")}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <section className="rounded-lg border border-base-300 bg-base-100 p-4">
          <div className="flex items-center gap-2">
            <Cpu className="size-5 text-primary" aria-hidden="true" />
            <h2 className="font-semibold">{t("settings.serverSettings.mlTitle")}</h2>
          </div>
          <div className="mt-4 divide-y divide-base-300">
            {mlCapabilities.map((capability) => (
              <div
                key={capability.key}
                className="flex flex-wrap items-center justify-between gap-3 py-3 first:pt-0 last:pb-0"
              >
                <div>
                  <div className="text-sm font-medium">{capability.label}</div>
                  <div className="mt-0.5 font-mono text-xs text-base-content/50">
                    {capability.tasks.join(" · ")}
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <span className={enabledBadgeClass(capability.enabled)}>
                    {capability.enabled
                      ? t("settings.serverSettings.enabled")
                      : t("settings.serverSettings.disabled")}
                  </span>
                  <span className={availabilityBadgeClass(capability.available)}>
                    {capability.available
                      ? t("settings.serverSettings.available")
                      : t("settings.serverSettings.unavailable")}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="rounded-lg border border-base-300 bg-base-100 p-4">
          <div className="flex items-center gap-2">
            <Bot className="size-5 text-primary" aria-hidden="true" />
            <h2 className="font-semibold">{t("settings.serverSettings.llmTitle")}</h2>
          </div>
          <div className="mt-4 space-y-3">
            <CapabilityRow
              label={t("settings.serverSettings.agentEnabled")}
              value={t(
                `settings.serverSettings.booleanValues.${capabilities.llm.agentEnabled ? "true" : "false"}`,
              )}
            />
            <CapabilityRow
              label={t("settings.serverSettings.configured")}
              value={t(
                `settings.serverSettings.booleanValues.${capabilities.llm.configured ? "true" : "false"}`,
              )}
            />
            <CapabilityRow
              label={t("settings.serverSettings.provider")}
              value={capabilities.llm.provider || t("common.na")}
            />
            <CapabilityRow
              label={t("settings.serverSettings.model")}
              value={capabilities.llm.modelName || t("common.na")}
            />
          </div>
        </section>
      </div>

      <BackendTable backends={runtime.backends ?? []} />
      <NodeTable nodes={runtime.nodes ?? []} discoveryState={discoveryState} />
    </div>
  );
}
