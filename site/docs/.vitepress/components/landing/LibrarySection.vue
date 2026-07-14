<script setup lang="ts">
import { Layers, Map } from '@lucide/vue'

const filterChips = ['照片', '视频', 'RAW', '评分', '日期', '地点', '相机', '镜头', '标签']
</script>

<template>
  <section id="library" class="library-section section-shell">
    <p class="section-kicker">为大规模媒体库设计的浏览与筛选</p>
    <h2 class="section-heading">所有线索，<br />都是筛选条件。</h2>
    <p class="library-lede">
      照片、视频、RAW、评分、日期、地点、相机、镜头、标签——
      从海量媒体中快速缩小范围，直到找到你想要的那一张。
    </p>
    <div class="filter-row" aria-label="可用筛选条件">
      <span v-for="chip in filterChips" :key="chip">{{ chip }}</span>
    </div>

    <div class="library-grid">
      <article class="library-card library-stack">
        <div class="library-icon"><Layers :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>进阶整理</p>
          <h3>相似的归一组，重复的看得见。</h3>
          <span>
            Live Photo、RAW + JPEG、连拍照片自动识别并堆叠；
            感知哈希看见相似与重复，把混乱整理成清晰的候选，让媒体库清爽起来。
          </span>
        </div>
        <div class="stack-visual" aria-hidden="true">
          <div class="stack-photo stack-photo-a"><span>RAW</span></div>
          <div class="stack-photo stack-photo-b"><span>JPEG</span></div>
          <div class="stack-photo stack-photo-c"><span>LIVE · ×3</span></div>
        </div>
      </article>

      <article class="library-card library-trace">
        <div class="library-icon"><Map :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>探索与回顾</p>
          <h3>让媒体库长出脉络。</h3>
          <span>
              无论是拍摄地点、使用的镜头，还是焦段参数、连拍节奏，流明集都能把照片背后的信息，变成帮你回顾旅程、复盘创作的线索。回顾一段旅程，复盘一次创作，或找回某个地点的工作素材，让照片串联成清晰的轨迹。
          </span>
        </div>
        <div class="map-visuals">
          <img
            class="reveal-image"
            src="/images/landing/product-world-map.png"
            alt="流明集时空地图真实截图"
            width="1403"
            height="857"
            loading="lazy"
          />
          <img
            class="reveal-image"
            src="/images/landing/product-stats.png"
            alt="流明集统计真实截图"
            width="1403"
            height="857"
            loading="lazy"
          />
        </div>
      </article>
    </div>
  </section>
</template>

<style>
.library-section {
  padding-top: 170px;
  padding-bottom: 250px;
}

.library-lede {
  max-width: 690px;
  margin: 56px 0 0 auto;
  color: var(--muted);
  font-size: clamp(18px, 1.6vw, 24px);
  line-height: 1.6;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 52px;
}

.filter-row span {
  padding: 9px 16px;
  border: 1px solid rgba(21, 21, 18, 0.24);
  border-radius: 999px;
  color: #4d4b45;
  font-size: 13px;
  font-weight: 600;
}

.library-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  gap: 18px;
  margin-top: 64px;
}

.library-card {
  position: relative;
  overflow: hidden;
  border-radius: 25px;
}

.library-stack {
  grid-column: span 5;
  display: flex;
  flex-direction: column;
  min-height: 460px;
  padding: 40px;
  background: var(--orange);
  color: #1b0e08;
}

.stack-visual {
  position: relative;
  height: clamp(150px, 14vw, 210px);
  margin-top: auto;
  padding-top: 40px;
}

.stack-photo {
  position: absolute;
  bottom: 0;
  width: 44%;
  aspect-ratio: 4 / 3;
  border: 1.5px solid rgba(27, 14, 8, 0.55);
  border-radius: 14px;
  box-shadow: 0 10px 26px rgba(27, 14, 8, 0.22);
}

.stack-photo span {
  position: absolute;
  top: 10px;
  left: 10px;
  padding: 4px 8px;
  border-radius: 6px;
  background: #1b0e08;
  color: #ffe8db;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.stack-photo-a {
  left: 2%;
  background: #f6f1e6;
  transform: rotate(-5deg);
}

.stack-photo-b {
  left: 27%;
  bottom: 10px;
  background: #e9ddc8;
  transform: rotate(2deg);
}

.stack-photo-c {
  left: 52%;
  background: #1b0e08;
  transform: rotate(6deg);
}

.stack-photo-c span {
  background: #f6f1e6;
  color: #1b0e08;
}

.library-trace {
  grid-column: span 7;
  display: grid;
  min-height: 460px;
  padding: 40px;
  background: #101824;
  color: #f2efe8;
}

.library-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 52px;
  border: 1px solid currentColor;
  border-radius: 999px;
}

.library-card p {
  margin: 0 0 16px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.library-card h3 {
  max-width: 630px;
  margin: 0;
  font-size: clamp(27px, 2.8vw, 43px);
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: -0.05em;
}

.library-card > div > span {
  display: block;
  max-width: 580px;
  margin-top: 22px;
  color: currentColor;
  opacity: 0.8;
  font-size: 15px;
  line-height: 1.65;
}

.map-visuals {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  justify-self: center;
  width: 100%;
  margin-top: 42px;
}

.map-visuals img {
  width: 100%;
  height: 200px;
  border-radius: 16px;
  object-fit: cover;
  object-position: center;
}

@media (max-width: 760px) {
  .library-section { padding-top: 100px; padding-bottom: 160px; }
  .library-lede { margin-top: 40px; font-size: 18px; }
  .filter-row { margin-top: 40px; }
  .library-grid { grid-template-columns: 1fr; margin-top: 46px; }
  .library-stack,
  .library-trace { grid-column: auto; min-height: auto; padding: 30px; }
  .library-icon { margin-bottom: 40px; }
  .stack-visual { height: 140px; margin-top: 34px; padding-top: 0; }
  .map-visuals { width: 100%; margin-top: 34px; }
  .map-visuals img { height: 180px; }
}
</style>
