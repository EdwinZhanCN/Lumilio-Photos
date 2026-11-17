import { useId, useState, type ReactNode, type CSSProperties } from "react";
import { ChevronDown, Server } from "lucide-react";
import type { LumenNodeRuntime } from "../../api/useLumenRuntime";
import { errorCodeLabel, nodeVerdict, stateLabel } from "../../model/capabilityVocabulary";
import { useI18n } from "@/lib/i18n";
import { InspectorFields, InspectorState } from "./MonitorInspector";
import { formatDiagnosticTime } from "../../model/time";

export interface OrbitCapability {
  key: string;
  label: string;
  tasks: string[];
}
const PAGE_SIZE = 3;
const COLORS = ["#7ab2f0", "#e8b960", "#f0699a", "#77bfa3", "#ab92e8", "#db966b"];
function nodeKey(node: LumenNodeRuntime) {
  return node.id ?? node.endpoint ?? "";
}
function supports(node: LumenNodeRuntime, capability: OrbitCapability) {
  return capability.tasks.some((task) => node.tasks?.some((entry) => entry.task === task));
}
// Identity-seeded color assignment survives polling, reordering, and remounts.
function nodeColor(key: string) {
  let hash = 0;
  for (const char of key) hash = (Math.imul(hash, 31) + char.charCodeAt(0)) | 0;
  return COLORS[(hash >>> 0) % COLORS.length];
}

export function CapabilityOrbit({
  nodes,
  capabilities,
  overview,
}: {
  nodes: LumenNodeRuntime[];
  capabilities: OrbitCapability[];
  overview: ReactNode;
}) {
  const { t } = useI18n();
  const panelID = useId();
  const [selectedID, setSelectedID] = useState<string>();
  const [page, setPage] = useState(0);
  const [panelOpen, setPanelOpen] = useState(true);
  const ordered = [...nodes].sort((a, b) => nodeKey(a).localeCompare(nodeKey(b)));
  const pages = Math.max(1, Math.ceil(ordered.length / PAGE_SIZE));
  const currentPage = Math.min(page, pages - 1);
  const visible = ordered.slice(currentPage * PAGE_SIZE, (currentPage + 1) * PAGE_SIZE);
  const selected = visible.find((node) => nodeKey(node) === selectedID);
  return (
    <div className="monitor-orbit-workspace relative isolate">
      <div className="monitor-orbit-stage">
        <div
          className="monitor-orbit"
          data-paused={!!selected || undefined}
          role="group"
          aria-label={t("monitor.capabilities.orbitLabel", "Lumilio Photos and Lumen nodes")}
        >
          <button
            type="button"
            className={`monitor-orbit-center btn btn-ghost h-auto p-2 ${!selected ? "btn-active" : ""}`}
            aria-pressed={!selected}
            aria-controls={panelID}
            onClick={() => {
              setSelectedID(undefined);
              setPanelOpen(true);
            }}
          >
            <img src="/logo.png" alt="" className="size-12 object-contain" />
            <span className="max-w-16 text-center text-xs font-semibold">
              {t("app.name", "Lumilio Photos")}
            </span>
          </button>
          {visible.map((node, index) => (
            <div
              key={nodeKey(node)}
              className="monitor-orbit-arm"
              style={
                {
                  "--node-color": nodeColor(nodeKey(node)),
                  "--orbit-delay": `${(-index * 54) / visible.length}s`,
                  "--orbit-angle": `${(index * 360) / visible.length}deg`,
                } as CSSProperties
              }
            >
              <div className="monitor-orbit-ring" aria-hidden />
              <div className="monitor-orbit-rider">
                <button
                  type="button"
                  className="btn btn-outline monitor-node h-auto flex-nowrap gap-2 px-3 py-3 text-left font-normal"
                  aria-pressed={node === selected}
                  data-active={nodeVerdict(node) === "active"}
                  onClick={() => {
                    setSelectedID(nodeKey(node));
                    setPanelOpen(true);
                  }}
                >
                  <Server
                    aria-hidden
                    className="size-5 shrink-0"
                    style={{ color: "var(--node-color)" }}
                  />
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold" title={nodeKey(node)}>
                      {nodeKey(node)}
                    </span>
                    <span className="block text-xs text-base-content/60">
                      {stateLabel(t, nodeVerdict(node))}
                    </span>
                  </span>
                </button>
              </div>
            </div>
          ))}
          {!nodes.length && (
            <p className="absolute inset-x-4 bottom-14 text-center text-sm text-base-content/60">
              {t("monitor.capabilities.noNodes", "No Lumen nodes found")}
            </p>
          )}
        </div>
        {pages > 1 && (
          <nav
            className="flex items-center justify-center gap-2 pb-3"
            aria-label={t("monitor.capabilities.nodePages", "Node pages")}
          >
            <button
              className="btn btn-ghost btn-sm"
              disabled={currentPage === 0}
              onClick={() => {
                setPage(currentPage - 1);
                setSelectedID(undefined);
              }}
            >
              {t("common.previous", "Previous")}
            </button>
            <span className="text-xs tabular-nums">
              {currentPage + 1} / {pages}
            </span>
            <button
              className="btn btn-ghost btn-sm"
              disabled={currentPage === pages - 1}
              onClick={() => {
                setPage(currentPage + 1);
                setSelectedID(undefined);
              }}
            >
              {t("common.next", "Next")}
            </button>
          </nav>
        )}
      </div>
      <aside className="monitor-orbit-panel card z-10 border border-base-300 bg-base-100 shadow-lg">
        <button
          className="btn btn-ghost h-auto min-h-12 justify-between gap-3 rounded-b-none px-5 py-4 text-left"
          aria-expanded={panelOpen}
          aria-controls={panelID}
          onClick={() => setPanelOpen(!panelOpen)}
        >
          <span className="min-w-0 flex-1 break-words whitespace-normal text-sm font-semibold">
            {selected
              ? nodeKey(selected)
              : t("monitor.capabilities.discoveryBackends", "Discovery backends")}
          </span>
          {selected && <InspectorState state={nodeVerdict(selected)} />}
          <ChevronDown aria-hidden className={`size-4 shrink-0 ${panelOpen ? "rotate-180" : ""}`} />
        </button>
        {panelOpen && (
          <div id={panelID} className="border-t border-base-200 p-5">
            {selected ? (
              <section
                aria-label={t("monitor.snapshot.selection", "Selected detail")}
                className="space-y-5"
              >
                {selected.error_code && (
                  <p className="text-sm text-warning">{errorCodeLabel(t, selected.error_code)}</p>
                )}
                <ul className="space-y-3 text-sm">
                  {capabilities
                    .filter((capability) => supports(selected, capability))
                    .map((capability) => (
                      <li key={capability.key}>{capability.label}</li>
                    ))}
                </ul>
                {!selected.tasks?.length && (
                  <p className="text-sm text-base-content/60">
                    {selected.compatibility === "pending"
                      ? t(
                          "monitor.capabilities.capabilityPending",
                          "Checking supported capabilities",
                        )
                      : t(
                          "monitor.capabilities.noAdvertisedTasks",
                          "No supported capabilities reported",
                        )}
                  </p>
                )}
                <details
                  key={nodeKey(selected)}
                  className="collapse collapse-arrow rounded-none border-t border-base-200"
                >
                  <summary className="collapse-title min-h-0 py-3 pl-0 text-sm">
                    {t("monitor.capabilities.nodeDetails", "Connection details")}
                  </summary>
                  <div className="collapse-content space-y-4 px-0 text-sm">
                    <InspectorFields
                      fields={[
                        [t("monitor.capabilities.endpoint", "Address"), selected.endpoint],
                        [
                          t("monitor.capabilities.transport", "Transport"),
                          stateLabel(t, selected.transport),
                        ],
                        [
                          t("monitor.capabilities.compatibility", "Compatibility"),
                          stateLabel(t, selected.compatibility),
                        ],
                        [t("monitor.capabilities.version", "Version"), selected.version],
                        [t("monitor.capabilities.runtime", "Runtime"), selected.runtime],
                        [t("monitor.capabilities.source", "Source"), selected.sources?.join(", ")],
                        [
                          t("monitor.capabilities.lastObserved", "Last observed"),
                          formatDiagnosticTime(selected.last_observed_at, "—"),
                        ],
                      ]}
                    />
                    <p className="break-all font-mono text-xs text-base-content/60">
                      {selected.tasks?.map((task) => `${task.service} / ${task.task}`).join(" · ")}
                    </p>
                  </div>
                </details>
              </section>
            ) : (
              overview
            )}
          </div>
        )}
      </aside>
    </div>
  );
}
