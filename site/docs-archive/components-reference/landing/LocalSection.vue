<template>
  <section class="local-section section-shell">
    <div class="local-copy">
      <p class="section-kicker">LOCAL FIRST</p>
      <h2 class="section-heading">你的媒体库，<br />不该以联网为前提。</h2>
      <p>
        使用本机 Ollama，或连接支持工具调用的 OpenAI 兼容端点；
        Lumen Hub 可留在本机，也可以发现局域网节点。选择权一直在你手里。
      </p>
    </div>
    <div class="local-stats">
      <div><strong>0</strong><span>必需云服务</span></div>
      <div><strong>4</strong><span>本地视觉模型</span></div>
      <div><strong>19</strong><span>Agent 真实工具</span></div>
    </div>
  </section>
</template>

<style>
.local-section {
  display: grid;
  grid-template-columns: 1.2fr 0.8fr;
  gap: 90px;
  padding-top: clamp(90px, 10vw, 150px);
  padding-bottom: clamp(90px, 10vw, 150px);
  border-radius: 34px;
  background: var(--night);
  color: #f2efe8;
}

.local-copy {
  padding-left: clamp(34px, 7vw, 100px);
}

.local-copy .section-kicker { color: #9a968d; }

.local-copy .section-heading {
  max-width: 780px;
  color: #f2efe8;
  font-size: clamp(48px, 6vw, 86px);
}

.local-copy > p:last-child {
  max-width: 670px;
  margin: 46px 0 0;
  color: #b9b5ac;
  font-size: 18px;
  line-height: 1.8;
}

.local-stats {
  display: grid;
  align-content: center;
  padding-right: clamp(34px, 7vw, 100px);
}

.local-stats div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 24px;
  padding: 24px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.16);
}

.local-stats strong {
  color: var(--acid);
  font-size: clamp(54px, 7vw, 95px);
  font-weight: 500;
  line-height: 1;
}

.local-stats span {
  color: #b9b5ac;
  font-size: 13px;
}

@media (max-width: 1100px) {
  .local-section { width: min(100% - 36px, 1480px); grid-template-columns: 1fr; gap: 60px; }
  .local-copy { padding-right: clamp(34px, 7vw, 100px); }
  .local-stats { padding: 0 clamp(34px, 7vw, 100px); }
}

@media (max-width: 760px) {
  .local-section { gap: 45px; border-radius: 24px; }
  .local-copy, .local-stats { padding-inline: 27px; }
  .local-stats strong { font-size: 54px; }
}
</style>
