<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useData, inBrowser } from 'vitepress'
import { watchEffect } from 'vue'
import ChineseLanding from '../components/ChineseLanding.vue'
import { defineAsyncComponent } from 'vue'

// The Atlas is a full-screen internal tool, built only when LUMILIO_ATLAS=1
// (`task atlas`); see docs/atlas/README.md in the repository.
declare const __LUMILIO_ATLAS__: boolean
const AtlasApp = __LUMILIO_ATLAS__ ? defineAsyncComponent(() => import('../atlas/AtlasApp.vue')) : null

const { lang, frontmatter } = useData()
watchEffect(() => {
  if (inBrowser) {
    document.cookie = `nf_lang=${lang.value}; expires=Mon, 1 Jan 2030 00:00:00 UTC; path=/`
  }
})
</script>

<template>
  <ClientOnly v-if="AtlasApp && frontmatter.atlas"><AtlasApp /></ClientOnly>
  <ChineseLanding v-else-if="frontmatter.landing === 'lumilio-zh'" />
  <DefaultTheme.Layout v-else />
</template>
