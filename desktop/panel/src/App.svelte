<script lang="ts">
  import { onMount } from "svelte";
  import TitleBar from "./components/TitleBar.svelte";
  import Wizard from "./components/wizard/Wizard.svelte";
  import Dashboard from "./components/dashboard/Dashboard.svelte";
  import { refreshState, store } from "./lib/store.svelte.ts";

  onMount(() => {
    void refreshState();
  });
</script>

<div class="flex h-screen flex-col overflow-hidden bg-surface text-base-content">
  <TitleBar />
  {#if store.data}
    {#if store.data.mode === "dashboard"}
      <Dashboard />
    {:else}
      <Wizard />
    {/if}
  {:else if store.error}
    <div class="flex flex-1 items-center justify-center p-8 text-sm text-error">{store.error}</div>
  {/if}
</div>
