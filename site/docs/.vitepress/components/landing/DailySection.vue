<script setup lang="ts">
const dailyFeatures = [
  ['01', '在线工作室', '轻量微调，五种版式的快速加框；专业精修仍交给你熟悉的工具'],
  ['02', '安全分享', '临时分享，保持控制：限时访问链接，无需账户即可查看指定内容'],
  ['03', '自动堆叠', 'Live Photo、RAW + JPEG、连拍与编辑版本'],
  ['04', '感知查重', '精确哈希与 64 位 DCT pHash'],
  ['05', '文件夹视图', '配合本地文件目录，赋能智能媒体库'],
  ['06', '智能分类', '文档、票据、插画等零样本分类'],
  ['07', '拍摄统计', '相机、镜头、焦段与热力图'],
  ['08', '时空地图', '用 GPS 重新浏览旅程'],
]
</script>

<template>
  <section class="daily-section section-shell">
    <div class="daily-heading">
      <p class="section-kicker">更多日常小工具</p>
      <h2 class="section-heading">聪明，但不喧宾夺主。</h2>
      <p class="daily-note">
        在媒体库内完成轻量日常处理；需要专业精修时，随时回到你熟悉的工具。
      </p>
    </div>
    <div class="daily-list">
      <article v-for="feature in dailyFeatures" :key="feature[0]">
        <span>{{ feature[0] }}</span>
        <h3>{{ feature[1] }}</h3>
        <p>{{ feature[2] }}</p>
      </article>
    </div>
  </section>
</template>

<style>
.daily-section {
  display: grid;
  grid-template-columns: minmax(300px, 0.7fr) minmax(0, 1.3fr);
  gap: clamp(50px, 9vw, 160px);
  padding-bottom: 240px;
}

.daily-heading {
  position: sticky;
  top: 130px;
  align-self: start;
}

.daily-heading .section-heading {
  font-size: clamp(46px, 5vw, 72px);
}

.daily-note {
  max-width: 420px;
  margin: 34px 0 0;
  color: var(--muted);
  font-size: 16px;
  line-height: 1.7;
}

.daily-list {
  border-top: 1px solid rgba(21, 21, 18, 0.22);
}

.daily-list article {
  display: grid;
  grid-template-columns: 54px 0.7fr 1fr;
  gap: 24px;
  align-items: center;
  min-height: 118px;
  border-bottom: 1px solid rgba(21, 21, 18, 0.22);
}

.daily-list article > span {
  color: #89857c;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.12em;
}

.daily-list h3,
.daily-list p {
  margin: 0;
}

.daily-list h3 {
  font-size: 23px;
  font-weight: 500;
  letter-spacing: -0.02em;
}

.daily-list p {
  color: var(--muted);
  font-size: 14px;
}

@media (max-width: 1100px) {
  .daily-section { grid-template-columns: 1fr; }
  .daily-heading { position: relative; top: auto; }
}

@media (max-width: 760px) {
  .daily-section { gap: 55px; padding-bottom: 160px; }
  .daily-list article { grid-template-columns: 36px 1fr; gap: 13px; padding: 22px 0; }
  .daily-list article p { grid-column: 2 / -1; }
}
</style>
