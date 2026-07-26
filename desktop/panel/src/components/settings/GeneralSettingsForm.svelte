<script lang="ts">
  import { untrack } from "svelte";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { refreshState, store } from "../../lib/store.svelte.ts";
  import RegionSelect from "../shared/RegionSelect.svelte";

  let {
    open,
    session,
    dirty = $bindable(false),
    saving = $bindable(false),
  }: {
    open: boolean;
    session: number;
    dirty?: boolean;
    saving?: boolean;
  } = $props();

  let region = $state("other");
  let originalRegion = "other";
  let error = $state("");

  $effect(() => {
    if (!open) return;
    void session;
    untrack(() => {
      region = store.data!.region === "cn" ? "cn" : "other";
      originalRegion = region;
      error = "";
    });
  });

  $effect(() => {
    dirty = region !== originalRegion;
  });

  export async function save(): Promise<boolean> {
    if (!dirty) return true;
    saving = true;
    error = "";
    try {
      await api.saveRegion(region);
      originalRegion = region;
      await refreshState();
      return true;
    } catch (cause) {
      error = cause instanceof Error ? cause.message : String(cause);
      return false;
    } finally {
      saving = false;
    }
  }
</script>

<div class="flex flex-col gap-3.5">
  <div>
    <h3 class="m-0 text-sm font-semibold">{t("generalSettings")}</h3>
    <p class="m-0 mt-1 text-xs text-muted">{t("generalSettingsHint")}</p>
  </div>
  <label class="flex flex-col items-start gap-2 text-xs font-semibold">
    <span>{t("regionLabel")}</span>
    <RegionSelect bind:region showHint={false} />
  </label>
  <p class="m-0 text-xs text-muted">
    {region === "cn" ? t("regionHintCn") : t("regionHintOther")}
  </p>
  {#if error}
    <div role="alert" class="alert alert-error alert-soft text-xs"><span>{error}</span></div>
  {/if}
</div>
