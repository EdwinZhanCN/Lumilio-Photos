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

  const configSummary = $derived(
    [lumen.preset, lumen.backend].filter(Boolean).join(" · ") || "—",
  );
  const versionSummary = $derived(
    updateAvailable
      ? `${lumen.installedVersion} → ${lumen.latestVersion}`
      : lumen.installedVersion || "—",
  );

  async function action(a: LumenAction) {
    await api.lumenAction(a);
    setTimeout(() => void refreshState(), 350);
  }
</script>

<div class="flex flex-col gap-3 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <div class="flex items-center justify-between gap-2.5">
    <div class="flex items-center gap-2.5">
      <span class="text-[14.5px] font-semibold">{t("hubService")}</span>
      <StatusBadge {status} />
    </div>
    <div class="flex items-center gap-2">
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
      <DropdownMenu.Root>
        <DropdownMenu.Trigger class="btn btn-sm px-2.5" aria-label="More actions">⋯</DropdownMenu.Trigger>
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
  </div>

  <div class="flex flex-wrap gap-x-6 gap-y-2">
    <div class="flex flex-col gap-0.5">
      <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("config")}</span>
      <span class="text-xs capitalize">{configSummary}</span>
    </div>
    <div class="flex flex-col gap-0.5">
      <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("version")}</span>
      <span class="text-xs">{versionSummary}</span>
    </div>
    <div class="flex min-w-0 flex-col gap-0.5">
      <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("modelCache")}</span>
      <span class="truncate font-mono text-xs" title={lumen.cacheDir}>{lumen.cacheDir || "—"}</span>
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
      <progress class="progress progress-primary h-[5px] w-full"></progress>
      <p class="m-0 text-xs text-muted">
        {lumen.phase === "downloading" && lumen.download
          ? t("hubDownloading", {
              model: lumen.download.model,
              done: formatBytes(lumen.download.bytesDone),
              total: "…",
            })
          : t("hubPreparing")}
      </p>
    {/if}
  {/if}

  {#if status === "failed"}
    <p class="m-0 text-xs text-error">{lumen.error || t("hubError")}</p>
  {/if}
</div>

<ConfigureDialog bind:open={configureOpen} />
