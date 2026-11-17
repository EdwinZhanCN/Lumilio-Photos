import { CapabilityOrbit } from "./CapabilityOrbit";
import { MonitorFrame } from "./MonitorFrame";
import { InspectorFields, InspectorState } from "./MonitorInspector";
import { AlertTriangle, CircleDashed, XCircle } from "lucide-react";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";
import { errorCodeLabel, type RuntimeState } from "../../model/capabilityVocabulary";
import { formatDiagnosticTime } from "../../model/time";
import { useLumenRuntime, type LumenBackendStatus } from "../../api/useLumenRuntime";

/** Diagnostic timestamps keep the longer style and an em-dash placeholder. */
function diagnosticTimestamp(value?: string): string {
  return formatDiagnosticTime(value, "—", { dateStyle: "medium", timeStyle: "medium" });
}

function DiscoveryHealth({ state }: { state: RuntimeState }) {
  const { t } = useI18n();
  if (state === "healthy") return null;
  const content = {
    disabled: {
      icon: XCircle,
      title: t("monitor.capabilities.discovery.disabled", "Lumen discovery is disabled"),
      className: "border-base-300 bg-base-200/50",
    },
    starting: {
      icon: CircleDashed,
      title: t("monitor.capabilities.discovery.starting", "Lumen discovery is starting"),
      className: "border-warning/40 bg-warning/10",
    },
    degraded: {
      icon: AlertTriangle,
      title: t("monitor.capabilities.discovery.degraded", "Lumen discovery is degraded"),
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
      </div>
    </div>
  );
}

function DiscoveryBackends({ backends }: { backends: LumenBackendStatus[] }) {
  const { t } = useI18n();
  return (
    <ul className="list divide-y divide-base-200">
      {backends.map((backend) => (
        <li key={backend.source ?? "unknown"} className="space-y-5 py-5 first:pt-0 last:pb-0">
          <div className="flex items-center justify-between gap-2">
            <span className="text-sm font-semibold">{backend.source ?? "—"}</span>
            <InspectorState state={backend.state} />
          </div>
          <InspectorFields
            fields={[
              [
                t("monitor.capabilities.scanResults", "Matched / rejected"),
                `${backend.matched_count ?? 0} / ${backend.rejected_count ?? 0}`,
              ],
              [
                t("monitor.capabilities.lastSuccess", "Last successful scan"),
                diagnosticTimestamp(backend.last_scan_succeeded_at),
              ],
              ...(backend.next_scan_at
                ? [
                    [
                      t("monitor.capabilities.nextScan", "Next scan"),
                      diagnosticTimestamp(backend.next_scan_at),
                    ] as [string, string],
                  ]
                : []),
            ]}
          />
          {backend.last_outcome &&
            backend.last_outcome !== "success" &&
            !backend.last_error_code && (
              <p className="text-sm text-warning">
                {backend.last_outcome === "timed_out"
                  ? t("monitor.capabilities.scanTimedOut", "Last scan timed out")
                  : backend.last_outcome === "cancelled"
                    ? t("monitor.capabilities.scanCancelled", "Last scan cancelled")
                    : t("monitor.capabilities.scanFailed", "Last scan failed")}
              </p>
            )}
          {backend.last_error_code && (
            <p className="text-sm text-error">
              {errorCodeLabel(t, backend.last_error_code)}
              {(backend.consecutive_failures ?? 0) > 0 &&
                ` · ${t("monitor.capabilities.consecutiveFailures", {
                  count: backend.consecutive_failures,
                  defaultValue: "{{count}} consecutive failures",
                })}`}
            </p>
          )}
        </li>
      ))}
    </ul>
  );
}

export function CapabilitiesMonitor() {
  const { t } = useI18n();
  const capabilityQuery = useCapabilities(5000);
  const runtimeQuery = useLumenRuntime(5000);
  const capabilities = capabilityQuery.capabilities;
  const runtime = runtimeQuery.data;

  const frame = {
    hasData: !!capabilities && !!runtime,
    isLoading: capabilityQuery.isLoading || runtimeQuery.isLoading,
    isFetching: capabilityQuery.isFetching || runtimeQuery.isFetching,
    error:
      capabilityQuery.isError || runtimeQuery.isError
        ? t("settings.serverSettings.capabilitiesError")
        : undefined,
    onRefresh: () => {
      void Promise.all([capabilityQuery.refetch(), runtimeQuery.refetch()]);
    },
    updatedAt: Math.min(capabilityQuery.dataUpdatedAt, runtimeQuery.dataUpdatedAt),
  };
  if (!capabilities || !runtime) return <MonitorFrame {...frame} />;

  const mlCapabilities = [
    {
      key: "semantic",
      label: t("settings.aiSettings.taskNames.semantic", "Image Semantic Analysis"),
      tasks: ["semantic_image_embed", "semantic_text_embed"],
    },
    {
      key: "face",
      label: t("settings.aiSettings.taskNames.face", "Person Recognition"),
      tasks: ["face_recognition"],
    },
    {
      key: "ocr",
      label: t("settings.aiSettings.taskNames.ocr", "OCR Text Recognition"),
      tasks: ["ocr"],
    },
    {
      key: "bioclip",
      label: t("settings.serverSettings.taskNames.bioClipClassify", "BioCLIP Species Recognition"),
      tasks: ["bioclip_classify"],
    },
  ];
  const discoveryState = runtime.discovery_state ?? capabilities.ml.discoveryState;

  return (
    <MonitorFrame {...frame}>
      <DiscoveryHealth state={discoveryState} />
      <CapabilityOrbit
        nodes={runtime.nodes ?? []}
        capabilities={mlCapabilities}
        overview={<DiscoveryBackends backends={runtime.backends ?? []} />}
      />
    </MonitorFrame>
  );
}
