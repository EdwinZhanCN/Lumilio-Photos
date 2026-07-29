<script lang="ts">
  import { DropdownMenu } from "bits-ui";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import {
    canOpenLumilio,
    canRestartRuntime,
    photosStatus,
    refreshState,
    runtimeBusy,
    store,
  } from "../../lib/store.svelte.ts";
  import type { RuntimePhase } from "../../lib/types.ts";
  import StatusBadge from "../shared/StatusBadge.svelte";

  let {
    onOpenSettings,
    onOpenErrorLog,
  }: {
    onOpenSettings: () => void;
    onOpenErrorLog: () => void;
  } = $props();

  const data = $derived(store.data!);
  const runtime = $derived(data.runtime);
  const network = $derived(runtime.network);
  const status = $derived(photosStatus(data));
  const busy = $derived(runtimeBusy(data));

  let actionError = $state("");
  let copied = $state(false);

  function phaseLabel(phase: RuntimePhase): string {
    return (
      {
        stopped: t("statusOff"),
        starting: t("statusStarting"),
        running: t("statusRunning"),
        restarting: t("statusRestarting"),
        failed: t("statusFailed"),
      } as const
    )[phase];
  }

  function stageLabel(stage?: string): string {
    if (!stage) return "—";
    return (
      {
        preparing: t("runtimeStagePreparing"),
        starting_server: t("runtimeStageStartingServer"),
        ready: t("runtimeStageReady"),
      } as Record<string, string>
    )[stage] ?? stage;
  }

  function networkModeLabel(): string {
    return (
      {
        local: t("networkLocal"),
        lan_http: t("networkLAN"),
      } as const
    )[network.mode] ?? "—";
  }

  function passkeySummary(): string {
    if (!network.passkeyEnabled) return t("passkeyDisabled");
    if (network.mode === "lan_http") return t("passkeyLocalOnly");
    return t("passkeyAvailable");
  }

  async function restart(): Promise<void> {
    actionError = "";
    try {
      await api.restartRuntime();
      await refreshState();
    } catch (cause) {
      actionError = cause instanceof Error ? cause.message : String(cause);
    }
  }

  async function copyOrigin(): Promise<void> {
    if (!runtime.browserURL) return;
    actionError = "";
    try {
      await navigator.clipboard.writeText(runtime.browserURL);
      copied = true;
      setTimeout(() => (copied = false), 1400);
    } catch (cause) {
      actionError = cause instanceof Error ? cause.message : String(cause);
    }
  }

  async function restoreLastKnownGood(): Promise<void> {
    actionError = "";
    try {
      await api.restoreRuntimeConfig();
      setTimeout(() => void refreshState(), 250);
    } catch (cause) {
      actionError = cause instanceof Error ? cause.message : String(cause);
    }
  }
</script>

<article class="card card-sm card-border h-full min-w-0 bg-raised">
  <div class="card-body gap-3.5">
    <div class="flex min-w-0 items-start justify-between gap-3">
      <div class="min-w-0">
        <div class="mb-1 flex flex-wrap items-center gap-2">
          <h2 class="card-title text-[15px]">{t("serverService")}</h2>
          <StatusBadge {status} />
        </div>
        <p class="m-0 text-xs text-muted">{phaseLabel(runtime.phase)}</p>
      </div>

      <DropdownMenu.Root>
        <DropdownMenu.Trigger
          class="btn btn-ghost btn-sm btn-square shrink-0"
          aria-label={t("moreActions")}
        >
          ⋯
        </DropdownMenu.Trigger>
        <DropdownMenu.Portal>
          <DropdownMenu.Content
            class="z-30 flex min-w-[190px] flex-col overflow-hidden rounded-lg border border-line bg-raised py-1 shadow-lg"
            align="end"
            sideOffset={4}
          >
            <DropdownMenu.Item
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none"
              onSelect={onOpenSettings}
            >
              {t("openServerSettings")}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              disabled={!runtime.browserURL}
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none data-[disabled]:cursor-not-allowed data-[disabled]:opacity-40"
              onSelect={() => void copyOrigin()}
            >
              {copied ? t("copied") : t("copyOrigin")}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none"
              onSelect={onOpenErrorLog}
            >
              {t("openErrorLog")}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>

    {#if runtime.phase === "failed"}
      <div role="alert" class="alert alert-error alert-soft items-start py-3 text-xs">
        <div class="min-w-0">
          <div class="font-semibold">{t("serverStartFailed")}</div>
          <dl class="mt-2 grid min-w-0 grid-cols-[auto_1fr] gap-x-3 gap-y-1">
            <dt class="text-error/75">{t("runtimeStage")}</dt>
            <dd class="m-0 min-w-0">{stageLabel(runtime.stage)}</dd>
            <dt class="text-error/75">{t("runtimeReason")}</dt>
            <dd class="m-0 min-w-0 break-words">
              {runtime.errorMessage || t("runtimeUnknownError")}
            </dd>
          </dl>
        </div>
      </div>
    {:else if runtime.errorCode === "candidate_rolled_back"}
      <div role="status" class="alert alert-warning alert-soft items-start py-3 text-xs">
        <div>
          <div class="font-semibold">{t("candidateRolledBack")}</div>
          <div class="mt-1 break-words">{runtime.errorMessage}</div>
        </div>
      </div>
    {:else if busy}
      <div class="flex items-center gap-2 text-xs text-muted">
        <span class="loading loading-bars loading-sm text-primary"></span>
        <span>{stageLabel(runtime.stage)}</span>
      </div>
    {/if}

    <dl class="grid min-w-0 grid-cols-2 gap-x-4 gap-y-2.5">
      <div class="min-w-0">
        <dt class="text-[10.5px] tracking-wide text-muted uppercase">{t("version")}</dt>
        <dd class="m-0 mt-0.5 text-xs tabular-nums">{data.version || "dev"}</dd>
      </div>
      <div class="min-w-0">
        <dt class="text-[10.5px] tracking-wide text-muted uppercase">{t("networkMode")}</dt>
        <dd class="m-0 mt-0.5 truncate text-xs" title={networkModeLabel()}>
          {networkModeLabel()}
        </dd>
      </div>
      <div class="col-span-2 min-w-0">
        <dt class="text-[10.5px] tracking-wide text-muted uppercase">{t("serverAddress")}</dt>
        <dd
          class="m-0 mt-0.5 truncate font-mono text-[11.5px]"
          title={runtime.browserURL}
        >
          {runtime.browserURL || "—"}
        </dd>
      </div>
      <div class="col-span-2 min-w-0">
        <dt class="text-[10.5px] tracking-wide text-muted uppercase">{t("passkeys")}</dt>
        <dd class="m-0 mt-0.5 text-xs">{passkeySummary()}</dd>
      </div>
    </dl>

    {#if actionError}
      <p role="alert" class="m-0 break-words text-xs text-error">{actionError}</p>
    {/if}

    <div class="card-actions mt-auto flex-wrap justify-start pt-1">
      <button
        class="btn btn-primary btn-sm"
        disabled={!canOpenLumilio(data)}
        onclick={() => void api.openApp()}
      >
        {t("openLumilio")}
      </button>
      <button
        class="btn btn-sm"
        disabled={!canRestartRuntime(data)}
        onclick={() => void restart()}
      >
        {runtime.phase === "failed" ? t("retry") : t("restart")}
      </button>
      {#if runtime.phase === "failed"}
        <button class="btn btn-ghost btn-sm" onclick={onOpenSettings}>
          {t("editConfiguration")}
        </button>
        <button
          class="btn btn-ghost btn-sm"
          disabled={!runtime.lastKnownGoodAvailable}
          title={!runtime.lastKnownGoodAvailable ? t("restoreUnavailable") : undefined}
          onclick={() => void restoreLastKnownGood()}
        >
          {t("restoreLastKnownGood")}
        </button>
      {/if}
    </div>
  </div>
</article>
