<script lang="ts">
  import { t } from "../../lib/i18n.svelte.ts";
  import type { Validation } from "../../lib/types.ts";

  let {
    label,
    path,
    validation,
    checking = false,
    readonly = false,
    onpick,
  }: {
    label: string;
    path: string;
    validation: Validation | null;
    checking?: boolean;
    readonly?: boolean;
    onpick?: () => void;
  } = $props();

  const state = $derived(checking || !validation ? "checking" : validation.writable ? "ok" : "bad");
  const dotColor = $derived(
    state === "ok" ? "bg-success" : state === "bad" ? "bg-error" : "bg-warning",
  );
  const message = $derived(
    state === "checking"
      ? t("checkingLocation")
      : state === "ok"
        ? t("writable", { free: validation?.freeHuman ?? "—" })
        : t("notWritable"),
  );
</script>

<div class="flex flex-col gap-2 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <div class="text-xs font-semibold tracking-wide text-muted uppercase">{label}</div>
  <div class="flex items-center gap-2">
    <div
      class="min-w-0 flex-1 truncate rounded-md border border-line bg-surface px-2.5 py-2 font-mono text-xs"
      title={path}
    >
      {path || "—"}
    </div>
    {#if !readonly && onpick}
      <button class="btn btn-sm whitespace-nowrap" onclick={onpick}>{t("choose")}</button>
    {/if}
  </div>
  <div class="flex items-center gap-2 text-xs text-muted">
    <span class={`h-[7px] w-[7px] shrink-0 rounded-full ${dotColor}`}></span>
    <span>{message}</span>
  </div>
</div>
