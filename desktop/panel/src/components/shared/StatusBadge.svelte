<script lang="ts">
  import { t } from "../../lib/i18n.svelte.ts";
  import type { ServiceStatus } from "../../lib/store.svelte.ts";

  let { status }: { status: ServiceStatus } = $props();

  const label = $derived(
    (
      {
        running: t("statusRunning"),
        starting: t("statusStarting"),
        off: t("statusOff"),
        failed: t("statusFailed"),
        disabled: t("statusDisabled"),
      } as const
    )[status],
  );

  const color = $derived(
    (
      {
        running: "text-success",
        starting: "text-warning",
        off: "text-muted",
        failed: "text-error",
        disabled: "text-muted",
      } as const
    )[status],
  );
</script>

<span
  class={`rounded-md bg-base-content/5 px-2 py-0.5 text-[11px] font-bold whitespace-nowrap ${color}`}
>
  {label}
</span>
