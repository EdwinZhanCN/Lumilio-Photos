<script setup lang="ts">
import { ArrowRight, GitFork } from '@lucide/vue'
</script>

<template>
  <section class="cta-section section-shell">
    <div class="cta-visual">
      <img class="reveal-image" src="/images/landing/scene-beach.webp" alt="海滩照片" />
      <img class="cta-bird" src="/images/landing/scene-bird.webp" alt="飞鸟照片" />
    </div>
    <div class="cta-copy">
      <p class="section-kicker">现在，把照片带回来</p>
      <h2>从一个普通目录开始。</h2>
      <p>免费、开源。先导入几张照片，再决定哪些智能能力值得打开。</p>
      <div class="cta-actions">
        <a class="button button-dark" href="/zh-cn/user-manual/introduction/installation">
          开始安装
          <ArrowRight :size="19" :stroke-width="1.8" />
        </a>
        <a class="button button-outline" href="/zh-cn/user-manual/introduction/">
          阅读文档
        </a>
        <a
          class="button button-outline"
          href="https://github.com/EdwinZhanCN/Lumilio-Photos"
          target="_blank"
          rel="noreferrer"
        >
          <GitFork :size="18" :stroke-width="1.7" />
          查看源码
        </a>
      </div>
    </div>
  </section>
</template>

<style>
.cta-section {
  display: grid;
  grid-template-columns: 0.86fr 1.14fr;
  min-height: 760px;
  margin-top: 180px;
  overflow: hidden;
  border-radius: 34px;
  background: var(--acid);
  color: #12140e;
}

.cta-visual {
  position: relative;
  min-height: 760px;
  overflow: hidden;
}

.cta-visual > img:first-child {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.cta-bird {
  position: absolute;
  right: -40px;
  bottom: 40px;
  width: 42%;
  aspect-ratio: 1;
  border: 7px solid var(--acid);
  border-radius: 999px;
  object-fit: cover;
}

.cta-copy {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: clamp(50px, 7vw, 110px);
}

.cta-copy .section-kicker { color: #506021; }

.cta-copy h2 {
  max-width: 800px;
  margin: 0;
  font-size: clamp(58px, 8vw, 118px);
  font-weight: 500;
  line-height: 1.02;
  letter-spacing: -0.065em;
}

.cta-copy > p:not(.section-kicker) {
  max-width: 610px;
  margin: 38px 0 0;
  color: #354217;
  font-size: 19px;
  line-height: 1.7;
}

.cta-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
  margin-top: 36px;
}

@media (max-width: 1100px) {
  .cta-section { grid-template-columns: 1fr; }
  .cta-visual { min-height: 520px; }
}

@media (max-width: 760px) {
  .cta-section { width: calc(100% - 28px); min-height: 0; margin-top: 110px; border-radius: 24px; }
  .cta-visual { min-height: 360px; }
  .cta-copy { padding: 52px 26px 58px; }
  .cta-copy h2 { font-size: clamp(56px, 17vw, 84px); }
  .cta-actions { align-items: stretch; flex-direction: column; }
  .cta-actions .button { width: 100%; }
}
</style>
