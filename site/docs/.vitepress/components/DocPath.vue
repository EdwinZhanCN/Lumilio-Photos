<script setup lang="ts">
import { ArrowRight, CircleAlert, Info, ShieldCheck } from "@lucide/vue";
import type { Component } from "vue";

withDefaults(
  defineProps<{
    title?: string;
    items?: string[];
    tone?: "info" | "safe" | "warning";
  }>(),
  {
    title: "在应用中找到它",
    items: () => [],
    tone: "info",
  },
);

const icons: Record<string, Component> = {
  info: Info,
  safe: ShieldCheck,
  warning: CircleAlert,
};
</script>

<template>
  <aside class="doc-path" :class="`doc-path--${tone}`">
    <div class="doc-path__title">
      <component :is="icons[tone]" :size="18" aria-hidden="true" />
      <strong>{{ title }}</strong>
    </div>
    <div v-if="items.length" class="doc-path__steps">
      <template v-for="(item, index) in items" :key="item">
        <span>{{ item }}</span>
        <ArrowRight v-if="index < items.length - 1" :size="15" aria-hidden="true" />
      </template>
    </div>
    <div v-else class="doc-path__body"><slot /></div>
  </aside>
</template>

<style scoped>
.doc-path {
  margin: 20px 0;
  padding: 14px 16px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  background: var(--vp-c-bg-soft);
}
.doc-path--safe { border-left: 3px solid var(--vp-c-green-1); }
.doc-path--warning { border-left: 3px solid var(--vp-c-warning-1); }
.doc-path__title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}
.doc-path__steps {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  color: var(--vp-c-text-2);
  font-size: 14px;
}
.doc-path__body { color: var(--vp-c-text-2); }
.doc-path__body :deep(p) { margin: 0; }
</style>
