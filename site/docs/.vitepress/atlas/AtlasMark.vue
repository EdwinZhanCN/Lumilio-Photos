<script setup lang="ts">
// The Atlas mark: three nodes joined by two edges — a path walked through a
// graph. Monochrome; it takes the surrounding text color.
defineProps<{ size?: number }>()
</script>

<template>
  <svg class="mark" :width="size ?? 22" :height="size ?? 22" viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <path d="M6 17.5 11.5 7.5 18 15" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.45" />
    <circle cx="6" cy="17.5" r="2.4" fill="currentColor" />
    <circle cx="11.5" cy="7.5" r="2.4" fill="var(--atlas-canvas)" stroke="currentColor" stroke-width="1.5" />
    <circle cx="18" cy="15" r="2.4" fill="currentColor" />
  </svg>
</template>
