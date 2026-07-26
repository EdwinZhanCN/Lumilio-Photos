<script lang="ts">
  import { api } from "../../lib/api.ts";
  import { t } from "../../lib/i18n.svelte.ts";
  import { store } from "../../lib/store.svelte.ts";

  const data = $derived(store.data!);

  const rows = $derived(
    [
      { label: t("pathMedia"), path: data.paths.storage },
      { label: t("pathLogs"), path: data.paths.logs },
      { label: t("pathBackups"), path: data.paths.backups },
      { label: t("pathAppData"), path: data.paths.appData },
      // Rendered only while a stale cache dir exists after a cache-path change.
      { label: t("pathOldCache"), path: data.lumen.previousCacheDir },
    ].filter((r): r is { label: string; path: string } => Boolean(r.path)),
  );
</script>

<section class="flex min-w-0 flex-col gap-1 border-t border-line pt-3">
  <span class="pb-1.5 text-[13.5px] font-semibold">{t("localPaths")}</span>
  {#each rows as row (row.label)}
    <div class="flex items-center gap-2.5 border-t border-line py-2">
      <span class="w-[150px] shrink-0 text-xs">{row.label}</span>
      <span class="min-w-0 flex-1 truncate font-mono text-[11.5px] text-muted" title={row.path}>
        {row.path}
      </span>
      <button
        class="link shrink-0 text-[11.5px] text-primary"
        onclick={() => void api.openPath(row.path)}
      >
        {t("reveal")}
      </button>
    </div>
  {/each}
</section>
