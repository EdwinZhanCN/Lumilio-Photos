<template>
  <section class="statement section-shell">
    <p class="section-kicker">不止是一个图库</p>
    <h2 class="section-heading">
      智能是可选项，
      <span class="inline-photo"><img src="/images/landing/scene-fox.webp" alt="红狐照片" width="1400" height="932" loading="lazy" /></span>
      掌控是默认值。
    </h2>
    <p class="statement-copy">
      AI 能力全部可开可关；媒体仓库支持自由编辑、随时导出。
      流明集把智能放进工作流，拒绝把媒体锁进复杂流程里。
    </p>
  </section>
</template>

<style>
.statement {
  padding-top: clamp(160px, 20vw, 310px);
  padding-bottom: clamp(150px, 19vw, 280px);
}

.statement-copy {
  max-width: 690px;
  margin: 64px 0 0 auto;
  color: var(--muted);
  font-size: clamp(19px, 2vw, 29px);
  line-height: 1.55;
}

@media (max-width: 760px) {
  .statement { padding-top: 170px; padding-bottom: 160px; }
  .statement-copy { margin-top: 42px; font-size: 18px; }
}
</style>
