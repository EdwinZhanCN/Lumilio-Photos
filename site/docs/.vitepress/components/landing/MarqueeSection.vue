<script setup lang="ts">
const items = ['SEMANTIC', 'OCR', 'PLACES', 'INSIGHTFACE', 'SIGLIP 2', 'BIOCLIP 2', 'AGENT TOOLS']
// 动画位移 -50%，轨道必须是完全相同的两半；每半重复 3 组，保证单半宽度超过超宽视口，循环无空隙。
const track = [...items, ...items, ...items, ...items, ...items, ...items]
</script>

<template>
  <section class="marquee" aria-label="流明集能力索引">
    <div class="marquee-track">
      <template v-for="(item, index) in track" :key="index">
        <span>{{ item }}</span><i></i>
      </template>
    </div>
  </section>
</template>

<style>
.marquee {
  overflow: hidden;
  border-block: 1px solid rgba(21, 21, 18, 0.2);
  background: var(--orange);
}

.marquee-track {
  display: flex;
  align-items: center;
  width: max-content;
  min-height: 76px;
  animation: landing-marquee 28s linear infinite;
}

.marquee span {
  padding-inline: 28px;
  color: #1b0e08;
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.marquee i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: #1b0e08;
}

@keyframes landing-marquee {
  to { transform: translateX(-50%); }
}

@media (max-width: 760px) {
  .marquee-track { min-height: 62px; }
}

@media (prefers-reduced-motion: reduce) {
  .marquee-track { animation: none; }
}
</style>
