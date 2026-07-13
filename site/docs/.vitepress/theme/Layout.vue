<script setup lang="ts">
import DefaultTheme from 'vitepress/theme'
import { useData, inBrowser } from 'vitepress'
import { watchEffect } from 'vue'
import ChineseLanding from '../components/ChineseLanding.vue'

const { lang, frontmatter } = useData()
watchEffect(() => {
  if (inBrowser) {
    document.cookie = `nf_lang=${lang.value}; expires=Mon, 1 Jan 2030 00:00:00 UTC; path=/`
  }
})
</script>

<template>
  <ChineseLanding v-if="frontmatter.landing === 'lumilio-zh'" />
  <DefaultTheme.Layout v-else />
</template>
