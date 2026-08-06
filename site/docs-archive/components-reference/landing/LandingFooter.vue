<template>
  <footer class="landing-footer section-shell">
    <div class="brand footer-brand">
      <img src="/logo.png" alt="流明集图标" />
      <span>流明集</span>
    </div>
    <p>给照片一个聪明、诚实、随时可以离开的家。</p>
    <div class="footer-links">
      <a href="/zh-cn/user-manual/introduction/">文档</a>
      <a href="/redoc-static.html">API</a>
      <a href="https://github.com/EdwinZhanCN/Lumilio-Photos">GitHub</a>
    </div>
  </footer>
</template>

<style>
.landing-footer {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  align-items: center;
  min-height: 190px;
  margin-top: 70px;
  border-top: 1px solid rgba(21, 21, 18, 0.2);
}

.landing-footer p {
  margin: 0;
  color: var(--muted);
  font-size: 14px;
  text-align: center;
}

.footer-links {
  display: flex;
  justify-content: flex-end;
  gap: 28px;
  color: var(--muted);
  font-size: 14px;
}

@media (max-width: 760px) {
  .landing-footer { grid-template-columns: 1fr; gap: 26px; padding: 48px 0; text-align: center; }
  .footer-brand { justify-self: center; }
  .footer-links { justify-content: center; }
}
</style>
