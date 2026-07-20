<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { store } from "../../lib/store.svelte.ts";
  import RegionSelect from "../shared/RegionSelect.svelte";

  let region = $state(store.data!.region === "cn" ? "cn" : "other");
</script>

<div class="flex flex-col gap-2.5 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <span class="text-[13.5px] font-semibold">{t("settings")}</span>
  <div class="flex flex-wrap items-center gap-3.5">
    <span class="text-xs font-semibold tracking-wide text-muted uppercase">{t("regionLabel")}</span>
    <RegionSelect bind:region showHint={false} onchange={(r) => void api.saveRegion(r)} />
  </div>
  <p class="m-0 text-xs text-muted">
    {region === "cn" ? t("regionHintCn") : t("regionHintOther")}
  </p>
</div>
