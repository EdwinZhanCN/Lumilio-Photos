<script setup lang="ts">
import { ref } from 'vue'
import { ChevronLeft, ChevronRight } from '@lucide/vue'

const activeProof = ref(0)

const proofs = [
  {
    kicker: '流式瀑布资源库',
    title: '上万媒体，无压力浏览',
    text: '同时提供瀑布流和紧凑网格两种模式，浏览海量媒体不在话下。',
    image: '/images/landing/product-library.png',
    alt: '流明集真实资源库截图',
  },
  {
    kicker: '文件夹视图',
    title: '配合自留区，享受智能，轻松应对多种工作流',
    text: '流明集保留了自由文件区域，在上传收件箱以外你可以在文件系统中自由操作',
    image: '/images/landing/view-folder.png',
    alt: '流明集文件夹视图截图',
  },
  {
    kicker: '服务监控',
    title: '实时监控服务状态，安全稳定不妥协',
    text: '流明集可以监控后台作业队列，AI/ML处理覆盖率和 Lumen Hub 节点发现',
    image: '/images/landing/system-monitor.png',
    alt: '流明集服务监控真实截图',
  },
  {
    kicker: '多样主题',
    title: '数十种不同精美主题，切换无压力',
    text: '流明集使用DaisyUI，提供了丰富的主题选择，以及流明集独有主题 lumilio 和 lumilio-dark。',
    image: '/images/landing/view-themes.png',
    alt: '流明集多样主题真实截图',
  },
]

function previousProof() {
  activeProof.value = (activeProof.value - 1 + proofs.length) % proofs.length
}

function nextProof() {
  activeProof.value = (activeProof.value + 1) % proofs.length
}
</script>

<template>
  <section class="proof-section section-shell">
    <div class="proof-header">
      <div>
        <p class="section-kicker">为真实工作流和多样性打造</p>
        <h2 class="section-heading">更多丰富内容，<br />都来自流明集。</h2>
      </div>
      <div class="proof-controls" aria-label="切换真实截图">
        <button type="button" aria-label="上一张截图" @click="previousProof">
          <ChevronLeft :size="20" :stroke-width="1.8" />
        </button>
        <span>{{ String(activeProof + 1).padStart(2, '0') }} / 04</span>
        <button type="button" aria-label="下一张截图" @click="nextProof">
          <ChevronRight :size="20" :stroke-width="1.8" />
        </button>
      </div>
    </div>

    <div class="proof-frame">
      <div class="proof-copy">
        <p>{{ proofs[activeProof].kicker }}</p>
        <h3>{{ proofs[activeProof].title }}</h3>
        <span>{{ proofs[activeProof].text }}</span>
        <div class="proof-dots" aria-label="截图页码">
          <button
            v-for="(_, index) in proofs"
            :key="index"
            type="button"
            :class="{ active: activeProof === index }"
            :aria-label="`查看第 ${index + 1} 张截图`"
            @click="activeProof = index"
          ></button>
        </div>
      </div>
      <div class="proof-image-wrap">
        <Transition name="proof-fade" mode="out-in">
          <img
            :key="proofs[activeProof].image"
            :src="proofs[activeProof].image"
            :alt="proofs[activeProof].alt"
            width="1403"
            height="857"
            loading="lazy"
          />
        </Transition>
      </div>
    </div>
  </section>
</template>

<style>
.proof-section {
  padding-top: 110px;
  padding-bottom: 250px;
}

.proof-header {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 40px;
  margin-bottom: 60px;
}

.proof-header .section-heading {
  max-width: 900px;
  font-size: clamp(46px, 5.5vw, 78px);
}

.proof-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  color: #68655f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.09em;
}

.proof-controls button,
.proof-dots button {
  appearance: none;
  border: 0;
  cursor: pointer;
}

.proof-controls button {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border: 1px solid rgba(21, 21, 18, 0.22);
  border-radius: 999px;
  background: transparent;
  color: var(--ink);
}

.proof-frame {
  display: grid;
  grid-template-columns: minmax(290px, 0.36fr) minmax(0, 1fr);
  min-height: 680px;
  overflow: hidden;
  border-radius: 30px;
  background: #d8d2c7;
}

.proof-copy {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  padding: clamp(32px, 5vw, 70px);
}

.proof-copy > p {
  margin: 0 0 22px;
  color: #6b675f;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.proof-copy h3 {
  margin: 0;
  font-size: clamp(32px, 3.5vw, 54px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.045em;
}

.proof-copy > span {
  margin-top: 28px;
  color: var(--muted);
  line-height: 1.7;
}

.proof-dots {
  display: flex;
  gap: 8px;
  margin-top: 54px;
}

.proof-dots button {
  width: 36px;
  height: 3px;
  background: #9e998f;
  transition: background 180ms ease, width 180ms ease;
}

.proof-dots button.active {
  width: 72px;
  background: var(--ink);
}

.proof-image-wrap {
  position: relative;
  min-height: 680px;
  overflow: hidden;
  background: #151512;
}

.proof-image-wrap img {
  width: 100%;
  height: 100%;
  min-height: 680px;
  object-fit: cover;
  object-position: center;
}

.proof-fade-enter-active,
.proof-fade-leave-active {
  transition: opacity 280ms ease, transform 320ms ease;
}

.proof-fade-enter-from,
.proof-fade-leave-to {
  opacity: 0;
  transform: scale(1.02);
}

@media (max-width: 1100px) {
  .proof-frame { grid-template-columns: 1fr; }
  .proof-copy { min-height: 420px; }
}

@media (max-width: 760px) {
  .proof-section { padding-top: 50px; padding-bottom: 160px; }
  .proof-header { align-items: flex-start; flex-direction: column; margin-bottom: 46px; }
  .proof-frame { min-height: 0; border-radius: 22px; }
  .proof-copy { min-height: 390px; padding: 28px; }
  .proof-image-wrap, .proof-image-wrap img { min-height: 300px; }
  .proof-image-wrap img { object-position: left center; }
}
</style>
