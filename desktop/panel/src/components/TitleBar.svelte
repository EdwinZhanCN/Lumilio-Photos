<script lang="ts">
  import { i18n, setLang, t } from "../lib/i18n.svelte.ts";
</script>

<!-- Drag strip under the transparent macOS titlebar (InvisibleTitleBarHeight:
     30). The real traffic lights overlay the left side; keep it clear. -->
<div
  class="titlebar-drag flex h-[30px] shrink-0 items-center justify-end gap-2 border-b border-line px-2.5"
>
  <span class="mr-auto pl-[76px] text-[11px] font-semibold tracking-wide text-muted select-none">
    Lumilio Photos · {t("tag")}
  </span>
  <button
    class="rounded-md border border-line px-2 py-0.5 text-[11px] font-semibold text-muted hover:text-base-content"
    onclick={() => setLang(i18n.lang === "en" ? "zh" : "en")}
  >
    {i18n.lang === "en" ? "中文" : "EN"}
  </button>
</div>
