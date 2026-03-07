import {
  Bot,
  Cpu,
  Loader2,
  Network,
  RefreshCcw,
  Sparkles,
  Workflow,
} from "lucide-react";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";

function availabilityBadgeClass(task: {
  enabled: boolean;
  available: boolean;
}) {
  if (!task.enabled) {
    return "badge badge-ghost";
  }

  return task.available ? "badge badge-success" : "badge badge-error";
}

function enabledBadgeClass(enabled: boolean) {
  return enabled ? "badge badge-success badge-outline" : "badge badge-ghost";
}

function CapabilityRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-sm">
      <span className="opacity-60">{label}</span>
      <span className="font-medium text-right">{value}</span>
    </div>
  );
}

export function CapabilitiesMonitor() {
  const { t } = useI18n();
  const query = useCapabilities(5000);
  const capabilities = query.capabilities;

  if (query.isLoading) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <Loader2 className="w-8 h-8 animate-spin mx-auto text-primary" />
        <p className="mt-3 text-sm opacity-60">{t("common.loading")}</p>
      </div>
    );
  }

  if (query.isError || !capabilities) {
    return (
      <div className="bg-base-100 rounded-lg shadow-sm p-6 text-center">
        <div className="text-warning text-sm">
          {t("settings.serverSettings.capabilitiesError")}
        </div>
      </div>
    );
  }

  const mlTasks = [
    {
      key: "clip-image",
      label: t("settings.serverSettings.taskNames.clipImageEmbed"),
      capability: capabilities.ml.tasks.clipImageEmbed,
    },
    {
      key: "clip-text",
      label: t("settings.serverSettings.taskNames.clipTextEmbed"),
      capability: capabilities.ml.tasks.clipTextEmbed,
    },
    {
      key: "ocr",
      label: t("settings.serverSettings.taskNames.ocr"),
      capability: capabilities.ml.tasks.ocr,
    },
    {
      key: "vlm",
      label: t("settings.serverSettings.taskNames.vlmGenerate"),
      capability: capabilities.ml.tasks.vlmGenerate,
    },
    {
      key: "face",
      label: t("settings.serverSettings.taskNames.faceDetectAndEmbed"),
      capability: capabilities.ml.tasks.faceDetectAndEmbed,
    },
  ];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-end">
        <button
          className="btn btn-ghost btn-sm"
          onClick={() => void query.refetch()}
          disabled={query.isFetching}
        >
          <RefreshCcw
            className={`w-4 h-4 ${query.isFetching ? "animate-spin" : ""}`}
          />
          {t("settings.serverSettings.refresh")}
        </button>
      </div>

      <div className="stats stats-vertical xl:stats-horizontal shadow-sm w-full">
        <div className="stat">
          <div className="stat-figure text-primary">
            <Workflow className="w-8 h-8" />
          </div>
          <div className="stat-title">
            {t("settings.serverSettings.autoMode")}
          </div>
          <div className="stat-value text-primary text-2xl">
            {t(
              `settings.serverSettings.autoModeValues.${capabilities.ml.autoMode}`,
            )}
          </div>
          <div className="stat-desc">
            {t("settings.serverSettings.taskAvailability")}
          </div>
        </div>

        <div className="stat">
          <div className="stat-figure text-info">
            <Network className="w-8 h-8" />
          </div>
          <div className="stat-title">
            {t("settings.serverSettings.discoveredNodes")}
          </div>
          <div className="stat-value text-info">
            {capabilities.ml.discoveredNodeCount}
          </div>
          <div className="stat-desc">
            {capabilities.ml.activeNodeCount}{" "}
            {t("settings.serverSettings.activeNodes").toLowerCase()}
          </div>
        </div>

        <div className="stat">
          <div className="stat-figure text-success">
            <Cpu className="w-8 h-8" />
          </div>
          <div className="stat-title">
            {t("settings.serverSettings.taskAvailability")}
          </div>
          <div className="stat-value text-success">
            {mlTasks.filter((task) => task.capability.available).length}
          </div>
          <div className="stat-desc">
            / {mlTasks.length}{" "}
            {t("settings.serverSettings.available").toLowerCase()}
          </div>
        </div>

        <div className="stat">
          <div className="stat-figure text-secondary">
            <Sparkles className="w-8 h-8" />
          </div>
          <div className="stat-title">
            {t("settings.serverSettings.llmTitle")}
          </div>
          <div className="stat-value text-secondary text-2xl">
            {capabilities.llm.agentEnabled
              ? t("settings.serverSettings.enabled")
              : t("settings.serverSettings.disabled")}
          </div>
          <div className="stat-desc">
            {capabilities.llm.provider || t("common.na")}
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
        <section className="bg-base-100 rounded-lg shadow-sm p-4 space-y-4">
          <div className="flex items-center gap-2">
            <Cpu className="w-5 h-5 text-primary" />
            <h2 className="text-lg font-semibold">
              {t("settings.serverSettings.mlTitle")}
            </h2>
          </div>

          <div className="space-y-2">
            {mlTasks.map(({ key, label, capability }) => (
              <div
                key={key}
                className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-base-300 px-3 py-2"
              >
                <span className="text-sm font-medium">{label}</span>
                <div className="flex flex-wrap items-center gap-2">
                  <span className={enabledBadgeClass(capability.enabled)}>
                    {capability.enabled
                      ? t("settings.serverSettings.enabled")
                      : t("settings.serverSettings.disabled")}
                  </span>
                  <span className={availabilityBadgeClass(capability)}>
                    {capability.available
                      ? t("settings.serverSettings.available")
                      : t("settings.serverSettings.unavailable")}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </section>

        <section className="bg-base-100 rounded-lg shadow-sm p-4 space-y-4">
          <div className="flex items-center gap-2">
            <Bot className="w-5 h-5 text-primary" />
            <h2 className="text-lg font-semibold">
              {t("settings.serverSettings.llmTitle")}
            </h2>
          </div>

          <div className="space-y-3">
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
    </div>
  );
}
