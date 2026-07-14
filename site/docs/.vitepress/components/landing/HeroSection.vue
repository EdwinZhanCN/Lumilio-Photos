<script setup lang="ts">
import { BookOpen, Download } from '@lucide/vue'
</script>

<template>
  <section class="hero section-shell">
    <div class="hero-copy">
      <p class="eyebrow">本地优先的媒体管理系统</p>
      <h1>看得快，<br /> 找得准，<br />懂你所拍。</h1>
      <p class="hero-lede">
        为大规模媒体库设计，从秒级浏览筛选，到真正理解照片内容。
      </p>
      <div class="hero-actions">
        <a class="button button-primary" href="/zh-cn/user-manual/introduction/installation">
          <Download :size="19" :stroke-width="1.8" />
          安装流明集
        </a>
        <a class="text-link" href="/zh-cn/user-manual/introduction/">
          <BookOpen :size="17" :stroke-width="1.8" />
          阅读用户手册
        </a>
      </div>
      <div class="hero-meta" aria-label="项目特性">
        <span>开源免费</span>
        <span>本地仓库</span>
        <span>macOS · Windows · Docker 多平台</span>
      </div>
    </div>

    <div class="hero-stage" aria-label="流明集真实应用场景">
      <!-- <div class="hero-stage-bar">
        <span class="live-dot"></span>
        <span>真实运行 · 25 张演示照片</span>
        <span class="hero-stage-index">LIBRARY / 01</span>
      </div> -->
      <div class="hero-screen">
        <img
          class="reveal-image"
          src="/images/landing/product-library.png"
          alt="流明集真实资源库界面"
          width="1403"
          height="857"
          fetchpriority="high"
        />
      </div>
      <figure class="floating-photo floating-photo-left">
        <img src="/images/landing/scene-lake.webp" alt="湖边木屋照片" width="1600" height="1067" />
      </figure>
      <figure class="floating-photo floating-photo-right">
        <img src="/images/landing/scene-portrait.webp" alt="人物肖像照片" width="1067" height="1600" />
      </figure>
    </div>
  </section>
</template>

<style>
.hero {
  display: grid;
  grid-template-columns: minmax(420px, 0.84fr) minmax(0, 1.16fr);
  gap: clamp(48px, 6vw, 110px);
  align-items: center;
  min-height: 100svh;
  padding-top: 136px;
  padding-bottom: 100px;
}

.hero-copy {
  position: relative;
  z-index: 2;
}

.hero h1 {
  max-width: 720px;
  margin: 0;
  font-size: clamp(64px, 6vw, 96px);
  font-weight: 500;
  line-height: 1.02;
  letter-spacing: -0.07em;
}

.hero-lede {
  max-width: 610px;
  margin: 38px 0 0;
  color: var(--muted);
  font-size: clamp(18px, 1.5vw, 23px);
  line-height: 1.65;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 18px;
  margin-top: 36px;
}

.hero-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 22px;
  margin-top: 56px;
  color: #77736a;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}

.hero-meta span:not(:last-child)::after {
  content: '/';
  margin-left: 22px;
  color: #aaa59c;
}

.hero-stage {
  position: relative;
  width: 100%;
  padding: 12px;
  border: 1px solid rgba(21, 21, 18, 0.17);
  border-radius: 24px;
  background: #d8d1c5;
  box-shadow: 0 36px 100px rgba(37, 32, 23, 0.18);
  transform: rotate(1.2deg);
}

/*.hero-stage-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 42px;
  padding: 0 10px;
  color: #4f4c46;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.07em;
  text-transform: uppercase;
}*/

.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 999px;
  background: #739928;
  box-shadow: 0 0 0 4px rgba(115, 153, 40, 0.13);
}

.hero-stage-index {
  margin-left: auto;
}

.hero-screen {
  overflow: hidden;
  border: 1px solid rgba(21, 21, 18, 0.2);
  border-radius: 15px;
  background: #faf8f2;
}

.hero-screen img {
  width: 100%;
  aspect-ratio: 1.63;
  object-fit: cover;
}

.floating-photo {
  position: absolute;
  overflow: hidden;
  margin: 0;
  border: 6px solid #f3efe6;
  box-shadow: 0 25px 70px rgba(28, 24, 17, 0.25);
}

.floating-photo img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.floating-photo-left {
  right: 72%;
  bottom: -56px;
  width: clamp(130px, 13vw, 208px);
  aspect-ratio: 0.82;
  transform: rotate(-6deg);
}

.floating-photo-right {
  right: -42px;
  top: -72px;
  width: clamp(118px, 11vw, 174px);
  aspect-ratio: 0.72;
  transform: rotate(5deg);
}

@media (max-width: 1100px) {
  .hero { grid-template-columns: 1fr; min-height: auto; padding-top: 160px; }
  .hero-copy { max-width: 900px; }
  .hero-stage { width: calc(100% - 30px); margin: 60px 0 0 30px; }
  .floating-photo-left { right: auto; left: -40px; }
}

@media (max-width: 760px) {
  .hero { gap: 10px; padding-top: 120px; padding-bottom: 90px; }
  .hero h1 { font-size: clamp(58px, 18vw, 82px); }
  .hero-lede { margin-top: 28px; font-size: 17px; }
  .hero-actions { align-items: flex-start; flex-direction: column; }
  .hero-meta { gap: 8px 12px; margin-top: 38px; }
  .hero-meta span::after { display: none; }
  .hero-stage { width: 100%; margin: 55px 0 0; padding: 7px; border-radius: 18px; transform: none; }
  /*.hero-stage-bar { min-height: 36px; font-size: 9px; }*/
  .hero-stage-index { display: none; }
  .floating-photo-left { left: -8px; bottom: -60px; width: 110px; }
  .floating-photo-right { right: -10px; top: -52px; width: 98px; }
}
</style>
