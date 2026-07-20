<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { anyServiceBusy, refreshState, store } from "../../lib/store.svelte.ts";
  import HubCard from "./HubCard.svelte";
  import LogPanel from "./LogPanel.svelte";
  import PathsPanel from "./PathsPanel.svelte";
  import PhotosCard from "./PhotosCard.svelte";
  import SettingsPanel from "./SettingsPanel.svelte";

  const data = $derived(store.data!);

  // Status is polled on demand (Refresh); the only automatic re-poll is while
  // a service is mid-transition, so "Starting" resolves without user action.
  $effect(() => {
    if (!anyServiceBusy(data)) return;
    const id = setInterval(() => void refreshState(), 3000);
    return () => clearInterval(id);
  });
</script>

<div class="flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-6 py-5">
  <div class="flex items-start justify-between gap-3">
    <div>
      <h1 class="m-0 text-[19px] font-semibold">{t("dashTitle")}</h1>
      <p class="m-0 mt-1 text-[13px] text-muted">{t("dashSub")}</p>
    </div>
    <div class="flex shrink-0 gap-2">
      <button class="btn btn-sm" onclick={() => void refreshState()}>{t("refresh")}</button>
      <button class="btn btn-primary btn-sm" onclick={() => void api.openApp()}>
        {t("openBrowser")}
      </button>
    </div>
  </div>

  <PhotosCard />
  <HubCard />
  <LogPanel />
  <SettingsPanel />
  <PathsPanel />
</div>
