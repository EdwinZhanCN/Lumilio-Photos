<script lang="ts">
  import { t } from "../../lib/i18n.svelte.ts";

  let {
    region = $bindable(),
    showHint = true,
    onchange,
  }: {
    region: string;
    showHint?: boolean;
    onchange?: (region: string) => void;
  } = $props();
</script>

<select
  class="select select-sm w-fit min-w-40 font-semibold"
  bind:value={region}
  onchange={() => onchange?.(region)}
>
  <option value="other">{t("regionOther")}</option>
  <option value="cn">{t("regionCn")}</option>
</select>
{#if showHint}
  <p class="m-0 text-xs text-muted">
    {region === "cn" ? t("regionHintCn") : t("regionHintOther")}
  </p>
{/if}
