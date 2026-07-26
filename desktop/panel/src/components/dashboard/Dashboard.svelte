<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import {
    anyServiceBusy,
    canOpenLumilio,
    refreshState,
    store,
  } from "../../lib/store.svelte.ts";
  import HubCard from "./HubCard.svelte";
  import ServerCard from "./ServerCard.svelte";
  import SettingsPanel from "./SettingsPanel.svelte";
  import StorageLocationsPanel from "./StorageLocationsPanel.svelte";
  import SupportPanel from "./SupportPanel.svelte";

  const data = $derived(store.data!);
  let supportOpen = $state(false);
  let supportSource = $state("app");

  function openSettings(): void {
    const panel = document.getElementById("desktop-settings");
    panel?.scrollIntoView({ behavior: "smooth", block: "start" });
    panel?.focus({ preventScroll: true });
  }

  function openErrorLog(): void {
    supportSource = "error";
    supportOpen = true;
  }

  // Status is polled on demand (Refresh); the only automatic re-poll is while
  // a service is mid-transition, so "Starting" resolves without user action.
  $effect(() => {
    if (!anyServiceBusy(data)) return;
    const id = setInterval(() => void refreshState(), 3000);
    return () => clearInterval(id);
  });
</script>

<main class="flex min-h-0 flex-1 flex-col gap-4 overflow-auto px-5 py-4 min-[700px]:px-6 min-[700px]:py-5">
  <header class="flex flex-wrap items-start justify-between gap-3">
    <div>
      <h1 class="m-0 text-[19px] font-semibold">{t("dashTitle")}</h1>
      <p class="m-0 mt-1 text-[13px] text-muted">{t("dashSub")}</p>
    </div>
    <div class="flex shrink-0 flex-wrap justify-end gap-2">
      <button class="btn btn-sm" onclick={() => void refreshState()}>{t("refresh")}</button>
      <button class="btn btn-sm" onclick={openSettings}>{t("settings")}</button>
      <button
        class="btn btn-primary btn-sm"
        disabled={!canOpenLumilio(data)}
        onclick={() => void api.openApp()}
      >
        {t("openLumilio")}
      </button>
    </div>
  </header>

  <section
    class="grid min-w-0 grid-cols-1 items-stretch gap-4 min-[700px]:grid-cols-2"
    aria-label={t("primaryRuntime")}
  >
    <ServerCard onOpenSettings={openSettings} onOpenErrorLog={openErrorLog} />
    <HubCard />
  </section>
  <StorageLocationsPanel />
  <SettingsPanel />
  <SupportPanel bind:open={supportOpen} bind:source={supportSource} />
</main>
