<script setup lang="ts">
import { Check, HardDrive, ShieldCheck } from '@lucide/vue'
</script>

<template>
  <section id="integrity" class="integrity-section section-shell">
    <p class="section-kicker">安全性与可信任体验</p>
    <h2 class="section-heading">
      AI 可以离线，
      <span class="inline-photo inline-photo-tall"><img src="/images/landing/scene-camera.webp" alt="相机照片" width="1200" height="1500" loading="lazy" /></span>
      <span class="integrity-claim">原件不能失控。</span>
    </h2>

    <div class="integrity-grid">
      <article class="integrity-card integrity-primary">
        <div class="integrity-icon"><HardDrive :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>普通目录就是仓库</p>
          <h3>随时看得见，随时得离开。</h3>
          <span>上传和自由工作区分开，两种方式随你选择。系统文件永远隔离，互不影响。</span>
        </div>
        <div class="integrity-shots">
          <img class="reveal-image" src="/images/landing/file-system.png" alt="普通目录即仓库的文件系统视图" width="1856" height="1410" loading="lazy" />
          <img class="reveal-image" src="/images/landing/system-files.png" alt="隔离的系统文件视图" width="2064" height="1096" loading="lazy" />
        </div>
      </article>
      <article class="integrity-card integrity-secondary">
        <div class="integrity-icon"><ShieldCheck :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>写入前先站稳</p>
          <h3>BLAKE3 高效哈希、暂存后提交、软删除。</h3>
          <span>导入时先校验再提交；移除记录不碰原件。</span>
        </div>
      </article>
      <article class="integrity-card integrity-tertiary">
        <div class="integrity-icon"><Check :size="25" :stroke-width="1.5" /></div>
        <div>
          <p>数据安全不妥协</p>
          <h3>多用户权限，多因素认证，数据库自动备份通通都要。</h3>
          <span>使用TOTP和Passkey认证，默认每 24 小时备份数据库，保留最近 14 份。</span>
        </div>
      </article>
    </div>
  </section>
</template>

<style>
.integrity-section {
  padding-bottom: 250px;
}

.integrity-section .section-heading {
  max-width: 1150px;
}

.integrity-claim {
  white-space: nowrap;
}

.integrity-grid {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-flow: dense;
  gap: 18px;
  margin-top: 92px;
}

.integrity-card {
  position: relative;
  overflow: hidden;
  border-radius: 25px;
}

.integrity-primary {
  grid-column: span 7;
  grid-row: span 2;
  display: grid;
  min-height: 680px;
  padding: 46px;
  background: #151512;
  color: #f2efe8;
}

.integrity-secondary,
.integrity-tertiary {
  grid-column: span 5;
  min-height: 331px;
  padding: 40px;
}

.integrity-secondary { background: var(--orange); color: #1b0e08; }
.integrity-tertiary { background: #cbd8f3; color: #101824; }

.integrity-icon {
  display: grid;
  place-items: center;
  width: 48px;
  height: 48px;
  margin-bottom: 52px;
  border: 1px solid currentColor;
  border-radius: 999px;
}

.integrity-card p {
  margin: 0 0 16px;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.integrity-card h3 {
  max-width: 630px;
  margin: 0;
  font-size: clamp(31px, 4vw, 60px);
  font-weight: 500;
  line-height: 1.1;
  letter-spacing: -0.05em;
}

.integrity-secondary h3,
.integrity-tertiary h3 {
  font-size: clamp(27px, 2.8vw, 43px);
}

.integrity-card span {
  display: block;
  max-width: 580px;
  margin-top: 22px;
  color: currentColor;
  opacity: 0.76;
  font-size: 15px;
  line-height: 1.65;
}

.integrity-shots {
  display: grid;
  grid-template-columns: 1fr 1fr;
  align-self: end;
  gap: 14px;
  margin-top: 48px;
}

.integrity-shots img {
  width: 100%;
  height: 300px;
  padding: 14px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  background: #1e1e1a;
  object-fit: contain;
}

@media (max-width: 760px) {
  .integrity-claim { white-space: normal; }
  .integrity-section { padding-bottom: 160px; }
  .integrity-grid { grid-template-columns: 1fr; margin-top: 58px; }
  .integrity-primary, .integrity-secondary, .integrity-tertiary { grid-column: auto; grid-row: auto; min-height: auto; padding: 30px; }
  .integrity-primary { min-height: 610px; }
  .integrity-icon { margin-bottom: 40px; }
  .integrity-shots { grid-template-columns: 1fr; }
  .integrity-shots img { height: 240px; }
}
</style>
