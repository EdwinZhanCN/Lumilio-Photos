<script lang="ts">
  import { DropdownMenu } from "bits-ui";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { formatBytes } from "../../lib/format.ts";
  import { hubStatus, hubUpdateAvailable, refreshState, store } from "../../lib/store.svelte.ts";
  import type { LumenAction } from "../../lib/types.ts";
  import StatusBadge from "../shared/StatusBadge.svelte";
  import ConfigureDialog from "./ConfigureDialog.svelte";

  const data = $derived(store.data!);
  const lumen = $derived(data.lumen);
  const status = $derived(hubStatus(data));
  const offLike = $derived(status === "off" || status === "disabled");
  const updateAvailable = $derived(hubUpdateAvailable(data));

  let configureOpen = $state(false);
  let actionError = $state("");

  const configSummary = $derived(
    [lumen.preset, lumen.backend].filter(Boolean).join(" · ") || "—",
  );
  const versionSummary = $derived(
    updateAvailable
      ? `${lumen.installedVersion} → ${lumen.latestVersion}`
      : lumen.installedVersion || "—",
  );

  async function action(a: LumenAction) {
    actionError = "";
    try {
      await api.lumenAction(a);
      setTimeout(() => void refreshState(), 350);
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
          <h2 class="card-title text-[15px]">{t("hubService")}</h2>
          <StatusBadge {status} />
        </div>
        <p class="m-0 text-xs text-muted">{t("hubSupervised")}</p>
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
            class="z-30 flex min-w-[160px] flex-col overflow-hidden rounded-lg border border-line bg-raised py-1 shadow-lg"
            align="end"
            sideOffset={4}
          >
            <DropdownMenu.Item
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none"
              onSelect={() => void action("restart")}
            >
              {t("restart")}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none"
              onSelect={() => (configureOpen = true)}
            >
              {t("configure")}
            </DropdownMenu.Item>
            <DropdownMenu.Item
              class="cursor-pointer px-3 py-2 text-left text-xs hover:bg-accent-soft focus:bg-accent-soft focus:outline-none"
              onSelect={() => void action("check")}
            >
              {t("checkUpdate")}
            </DropdownMenu.Item>
          </DropdownMenu.Content>
        </DropdownMenu.Portal>
      </DropdownMenu.Root>
    </div>

    <div class="grid min-w-0 grid-cols-2 gap-x-4 gap-y-2.5">
      <div class="min-w-0">
        <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("config")}</span>
        <div class="mt-0.5 truncate text-xs capitalize" title={configSummary}>{configSummary}</div>
      </div>
      <div class="min-w-0">
        <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("version")}</span>
        <div class="mt-0.5 truncate text-xs tabular-nums" title={versionSummary}>{versionSummary}</div>
      </div>
      <div class="col-span-2 min-w-0">
        <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("modelCache")}</span>
        <div class="mt-0.5 truncate font-mono text-[11.5px]" title={lumen.cacheDir}>
          {lumen.cacheDir || "—"}
        </div>
      </div>
    </div>

    {#if status === "starting"}
      {#if lumen.download && lumen.download.bytesTotal > 0}
        <progress
          class="progress progress-primary h-[5px] w-full"
          value={lumen.download.bytesDone}
          max={lumen.download.bytesTotal}
        ></progress>
        <p class="m-0 text-xs text-muted">
          {t("hubDownloading", {
            model: lumen.download.model,
            done: formatBytes(lumen.download.bytesDone),
            total: formatBytes(lumen.download.bytesTotal),
          })}
        </p>
      {:else}
        <div class="flex items-center gap-2 text-xs text-muted">
          <span class="loading loading-bars loading-sm text-primary"></span>
          <span>
            {lumen.phase === "downloading" && lumen.download
              ? t("hubDownloading", {
                  model: lumen.download.model,
                  done: formatBytes(lumen.download.bytesDone),
                  total: "…",
                })
              : t("hubPreparing")}
          </span>
        </div>
      {/if}
    {/if}

    {#if status === "failed"}
      <p role="alert" class="m-0 break-words text-xs text-error">
        {lumen.error || t("hubError")}
      </p>
    {/if}
    {#if actionError}
      <p role="alert" class="m-0 break-words text-xs text-error">{actionError}</p>
    {/if}

    <div class="card-actions mt-auto flex-wrap justify-start pt-1">
      <button
        class={`btn btn-sm ${offLike ? "btn-primary" : ""}`}
        onclick={() => void action(offLike ? "enable" : "disable")}
      >
        {offLike ? t("enable") : t("turnOff")}
      </button>
      {#if updateAvailable}
        <button class="btn btn-outline btn-primary btn-sm" onclick={() => void action("update")}>
          {t("update")}
        </button>
      {/if}
      <button class="btn btn-ghost btn-sm" onclick={() => (configureOpen = true)}>
        {t("configure")}
      </button>
    </div>
  </div>
</article>

<ConfigureDialog bind:open={configureOpen} />
