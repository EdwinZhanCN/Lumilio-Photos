<script setup lang="ts">
import { Bird, ScanText, Users } from '@lucide/vue'
</script>

<template>
  <section id="understand" class="understand-section section-shell">
    <p class="section-kicker">本地媒体理解服务</p>
    <h2 class="section-heading">本地 AI 懂你所拍，<br /> 隐私不‘跑’远。</h2>
    <p class="understand-lede">
      语义搜索让照片找得到，智能分类让照片分得开，画质与审美排序帮你挑得出。
      所有理解都在本地完成，结果只存在你的媒体库。
    </p>

    <div class="understand-grid">
      <article class="understand-card understand-people">
        <div class="understand-icon"><Users :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>本地人物识别</p>
          <h3>越整理，越准确。</h3>
          <span>
            自动聚合同一人物的照片，支持手动合并与命名。
            所有修正都保存在你的本地媒体库中，人物相册随着使用持续变得更可靠。
          </span>
        </div>
        <img
          class="reveal-image"
          src="/images/landing/product-people.png"
          alt="流明集人物聚类真实截图"
          width="1403"
          height="857"
          loading="lazy"
        />
      </article>

      <article class="understand-card understand-ocr">
        <div class="understand-icon"><ScanText :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>照片里的文字</p>
          <h3>截图、票据、文档，<br/> 不再‘沉默’。</h3>
          <span>OCR 让照片里的文字也能被搜索和复用;一段地址、一张票据、一页白板，随手找回。</span>
        </div>
      </article>

      <article class="understand-card understand-bio">
        <div class="understand-icon"><Bird :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>BioCLIP 物种识别</p>
          <h3>自然摄影，轻松记录。</h3>
          <span>在本地识别鸟类、野生动物与植物，生物图鉴轻松搞定。</span>
        </div>
        <img
          class="understand-bird"
          src="/images/landing/scene-bird.webp"
          alt="飞鸟照片"
          loading="lazy"
        />
      </article>
    </div>
  </section>
</template>

<style>
.understand-section {
  padding-bottom: 250px;
}

.understand-lede {
  max-width: 690px;
  margin: 56px 0 0 auto;
  color: var(--muted);
  font-size: clamp(18px, 1.6vw, 24px);
  line-height: 1.6;
}

.understand-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-flow: dense;
  gap: 18px;
  margin-top: 92px;
}

.understand-card {
  position: relative;
  overflow: hidden;
  border-radius: 25px;
}

.understand-people {
  grid-column: span 7;
  grid-row: span 2;
  display: grid;
  min-height: 680px;
  padding: 46px;
  background: #172116;
  color: #f2efe8;
}

.understand-ocr,
.understand-bio {
  grid-column: span 5;
  min-height: 331px;
  padding: 40px;
}

.understand-ocr {
  background: var(--paper-muted);
  color: var(--ink);
}

.understand-bio {
  background: #dfe8c4;
  color: #1c250d;
}

.understand-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 52px;
  border: 1px solid currentColor;
  border-radius: 999px;
}

.understand-card p {
  margin: 0 0 16px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.understand-card h3 {
  max-width: 630px;
  margin: 0;
  font-size: clamp(31px, 4vw, 60px);
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: -0.05em;
}

.understand-ocr h3,
.understand-bio h3 {
  font-size: clamp(27px, 2.8vw, 43px);
}

.understand-card > div + div span,
.understand-card span {
  display: block;
  max-width: 580px;
  margin-top: 22px;
  color: currentColor;
  opacity: 0.76;
  font-size: 15px;
  line-height: 1.65;
}

.understand-people > img {
  align-self: end;
  width: 100%;
  height: 270px;
  margin-top: 48px;
  border-radius: 16px;
  object-fit: cover;
  object-position: center top;
}

.understand-bio .understand-bird {
  position: absolute;
  right: -34px;
  bottom: -34px;
  width: clamp(130px, 12vw, 190px);
  aspect-ratio: 1;
  border: 6px solid #dfe8c4;
  border-radius: 999px;
  object-fit: cover;
}

.understand-bio > div span {
  max-width: 420px;
}

@media (max-width: 760px) {
  .understand-section { padding-bottom: 160px; }
  .understand-lede { margin-top: 40px; font-size: 18px; }
  .understand-grid { grid-template-columns: 1fr; margin-top: 58px; }
  .understand-people,
  .understand-ocr,
  .understand-bio { grid-column: auto; grid-row: auto; min-height: auto; padding: 30px; }
  .understand-people { min-height: 610px; }
  .understand-icon { margin-bottom: 40px; }
  .understand-bio { padding-bottom: 120px; }
  .understand-bio .understand-bird { right: -22px; bottom: -22px; width: 120px; }
}
</style>
