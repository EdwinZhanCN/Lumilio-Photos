<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { photosStatus, store } from "../../lib/store.svelte.ts";
  import StatusBadge from "../shared/StatusBadge.svelte";

  const data = $derived(store.data!);
  const status = $derived(photosStatus(data));
</script>

<div class="flex flex-col gap-3 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <div class="flex items-center justify-between gap-2.5">
    <div class="flex items-center gap-2.5">
      <span class="text-[14.5px] font-semibold">{t("photosService")}</span>
      <StatusBadge {status} />
    </div>
    <button class="btn btn-sm" onclick={() => void api.openApp()}>{t("openBrowser")}</button>
  </div>

  <div class="flex flex-wrap gap-x-6 gap-y-2">
    <div class="flex flex-col gap-0.5">
      <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("version")}</span>
      <span class="text-xs">{data.version || "dev"}</span>
    </div>
    <div class="flex flex-col gap-0.5">
      <span class="text-[10.5px] tracking-wide text-muted uppercase">{t("address")}</span>
      <span class="font-mono text-xs">{data.serverURL || "—"}</span>
    </div>
  </div>

  {#if status === "starting"}
    <progress class="progress progress-primary h-[5px] w-full"></progress>
    <p class="m-0 text-xs text-muted">{t("startingNote")}</p>
  {/if}
</div>
