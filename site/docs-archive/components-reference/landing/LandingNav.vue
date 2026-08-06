<script setup lang="ts">
import { ArrowUpRight } from '@lucide/vue'

function jumpTo(sectionId: string) {
  document.getElementById(sectionId)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.history.replaceState(null, '', `#${sectionId}`)
}
</script>

<template>
  <header class="landing-nav" aria-label="主导航">
    <a class="brand" href="/zh-cn/" aria-label="流明集中文首页">
      <img src="/logo.png" alt="流明集图标" width="512" height="566" />
      <span>流明集</span>
    </a>
    <nav class="nav-links" aria-label="页面章节">
      <a href="#library" @click.prevent="jumpTo('library')">媒体库</a>
      <a href="#understand" @click.prevent="jumpTo('understand')">AI 理解</a>
      <a href="#agent" @click.prevent="jumpTo('agent')">Agent</a>
      <a href="#lumen" @click.prevent="jumpTo('lumen')">Lumen</a>
      <a href="#integrity" @click.prevent="jumpTo('integrity')">完整性</a>
    </nav>
    <a class="nav-action" href="/zh-cn/user-manual/introduction/installation">
      开始使用
      <ArrowUpRight :size="17" :stroke-width="1.8" />
    </a>
  </header>
</template>

<style>
.landing-nav {
  position: fixed;
  z-index: 100;
  top: 18px;
  left: 50%;
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  width: min(calc(100% - 36px), 1460px);
  min-height: 66px;
  padding: 10px 12px 10px 16px;
  border: 1px solid rgba(21, 21, 18, 0.14);
  border-radius: 18px;
  background: rgba(242, 239, 232, 0.92);
  box-shadow: 0 18px 60px rgba(39, 37, 31, 0.12);
  backdrop-filter: blur(16px);
  transform: translateX(-50%);
}

.nav-links {
  display: flex;
  align-items: center;
  gap: 32px;
  color: #4d4b45;
  font-size: 14px;
  font-weight: 500;
}

.nav-action {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  justify-self: end;
  min-height: 44px;
  padding: 0 18px;
  border-radius: 12px;
  background: var(--night);
  color: #f7f4ed !important;
  font-size: 14px;
  font-weight: 600;
}

.nav-action:hover {
  transform: translateY(-2px);
}

@media (max-width: 1100px) {
  .landing-nav { grid-template-columns: 1fr auto; }
  .nav-links { display: none; }
}

@media (max-width: 760px) {
  .landing-nav { top: 10px; width: calc(100% - 20px); min-height: 58px; border-radius: 15px; }
  .brand span { font-size: 14px; }
  .brand img { width: 30px; height: 30px; }
  .nav-action { min-height: 40px; padding: 0 13px; font-size: 12px; }
}
</style>
