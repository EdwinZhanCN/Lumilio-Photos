<script lang="ts">
  import { t } from "../../lib/i18n.svelte.ts";
  import { store } from "../../lib/store.svelte.ts";
  import LogPanel from "./LogPanel.svelte";
  import PathsPanel from "./PathsPanel.svelte";

  let {
    open = $bindable(false),
    source = $bindable("app"),
  }: {
    open?: boolean;
    source?: string;
  } = $props();

  const data = $derived(store.data!);

  $effect(() => {
    if (data.runtime.phase === "failed" || data.lumen.state === "failed") {
      open = true;
    }
  });
</script>

<section aria-labelledby="support-heading">
  <details
    bind:open
    class="collapse collapse-arrow border border-line bg-raised open:pb-1"
  >
    <summary class="collapse-title min-h-0 px-4 py-3.5">
      <span class="block text-[13.5px] font-semibold" id="support-heading">
        {t("supportTitle")}
      </span>
      <span class="mt-0.5 block pr-5 text-xs font-normal text-muted">
        {t("supportHint")}
      </span>
    </summary>
    <div class="collapse-content px-4 pb-4">
      <div class="grid min-w-0 gap-4">
        <LogPanel bind:source />
        <PathsPanel />
      </div>
    </div>
  </details>
</section>
