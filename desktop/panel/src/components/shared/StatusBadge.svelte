<script lang="ts">
  import { t } from "../../lib/i18n.svelte.ts";
  import type { ServiceStatus } from "../../lib/store.svelte.ts";

  let { status }: { status: ServiceStatus } = $props();

  const label = $derived(
    (
      {
        running: t("statusRunning"),
        starting: t("statusStarting"),
        restarting: t("statusRestarting"),
        off: t("statusOff"),
        failed: t("statusFailed"),
        disabled: t("statusDisabled"),
      } as const
    )[status],
  );

  const color = $derived(
    (
      {
        running: "badge-success",
        starting: "badge-warning",
        restarting: "badge-warning",
        off: "badge-ghost",
        failed: "badge-error",
        disabled: "badge-ghost",
      } as const
    )[status],
  );
</script>

<span class={`badge badge-soft badge-sm text-[11px] font-bold whitespace-nowrap ${color}`}>
  {label}
</span>
