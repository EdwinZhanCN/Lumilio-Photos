<script lang="ts">
  import { Tabs } from "bits-ui";
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";

  // Tab ids match the Go log sources (see handleDashboardLog).
  const sources = [
    { id: "app", label: () => t("logApp") },
    { id: "error", label: () => t("logErrors") },
    { id: "postgres", label: () => t("logDatabase") },
    { id: "lumen", label: () => t("logLumen") },
  ];

  let source = $state("app");
  let content = $state("");
  let path = $state("");
  let unreadable = $state(false);
  let pane = $state<HTMLDivElement | null>(null);

  async function load() {
    unreadable = false;
    try {
      const r = await api.log(source);
      content = r.content;
      path = r.path;
      // Jump to the newest entries once the pane has re-rendered.
      requestAnimationFrame(() => {
        if (pane) pane.scrollTop = pane.scrollHeight;
      });
    } catch {
      content = "";
      unreadable = true;
    }
  }

  $effect(() => {
    void source;
    void load();
  });
</script>

<div class="flex flex-col gap-2.5 rounded-[10px] border border-line bg-raised px-4 py-3.5">
  <div class="flex items-center justify-between">
    <span class="text-[13.5px] font-semibold">{t("diagnostics")}</span>
    <button class="btn btn-ghost btn-sm text-muted" onclick={() => void load()}>
      {t("refresh")}
    </button>
  </div>

  <Tabs.Root bind:value={source}>
    <Tabs.List class="flex gap-1 border-b border-line pb-2">
      {#each sources as s (s.id)}
        <Tabs.Trigger
          value={s.id}
          class="rounded-md px-2.5 py-1 text-xs font-semibold text-muted focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none data-[state=active]:bg-accent-soft data-[state=active]:text-base-content"
        >
          {s.label()}
        </Tabs.Trigger>
      {/each}
    </Tabs.List>
  </Tabs.Root>

  <div
    bind:this={pane}
    class="h-[240px] overflow-auto rounded-[7px] border border-line bg-surface px-3 py-2.5"
  >
    {#if unreadable}
      <div class="px-0.5 py-2.5 text-xs text-muted">{t("logUnreadable")}</div>
    {:else if content === ""}
      <div class="px-0.5 py-2.5 text-xs text-muted">{t("logEmpty")}</div>
    {:else}
      <pre class="m-0 font-mono text-[11.5px] leading-relaxed whitespace-pre-wrap">{content}</pre>
    {/if}
  </div>

  <div class="font-mono text-[11px] text-muted">{path || "—"}</div>
</div>
