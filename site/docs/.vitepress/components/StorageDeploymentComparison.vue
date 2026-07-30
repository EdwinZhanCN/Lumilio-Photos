<script setup lang="ts">
import {
  ArrowDown,
  Container,
  Database,
  FolderCheck,
  HardDrive,
  Monitor,
  Server,
  Settings,
} from "@lucide/vue";
</script>

<template>
  <div class="storage-comparison">
    <div class="storage-comparison__grid">
      <article
        class="deployment deployment--desktop"
        aria-labelledby="storage-desktop-title"
      >
        <header class="deployment__header">
          <span class="deployment__icon" aria-hidden="true">
            <Monitor :size="22" />
          </span>
          <div>
            <p class="deployment__eyebrow">Desktop</p>
            <h4 id="storage-desktop-title">多个存储位置，每处可有多个资源库</h4>
          </div>
        </header>

        <ol class="storage-flow" aria-label="Desktop 存储路径">
          <li class="storage-flow__step">
            <Settings :size="18" aria-hidden="true" />
            <div>
              <strong>Desktop 控制面板</strong>
              <span>获得操作系统的目录访问权限</span>
            </div>
          </li>
          <li class="storage-flow__arrow" aria-hidden="true">
            <ArrowDown :size="16" />
            <span>自动提供默认位置</span>
          </li>
          <li class="storage-flow__step">
            <HardDrive :size="18" aria-hidden="true" />
            <div>
              <strong><code>&lt;应用数据目录&gt;/storage</code></strong>
              <span>首次设置期间位置不可更换，完成后可添加其他位置</span>
            </div>
          </li>
          <li class="storage-flow__arrow" aria-hidden="true">
            <ArrowDown :size="16" />
            <span>创建资源库</span>
          </li>
          <li class="storage-flow__step storage-flow__step--result">
            <FolderCheck :size="18" aria-hidden="true" />
            <div>
              <strong><code>&lt;存储位置&gt;/&lt;资源库目录&gt;</code></strong>
              <span>每个存储位置都可以包含多个独立资源库</span>
            </div>
          </li>
        </ol>

        <dl class="deployment__facts">
          <div>
            <dt>存储位置</dt>
            <dd>一个默认位置，还可以添加多个外接磁盘或网络卷。</dd>
          </div>
          <div>
            <dt>添加方式</dt>
            <dd>在 Desktop 控制面板中选择目录；每个位置都有独立的 <code>.lumilioroot</code>。</dd>
          </div>
        </dl>
      </article>

      <article
        class="deployment deployment--server"
        aria-labelledby="storage-server-title"
      >
        <header class="deployment__header">
          <span class="deployment__icon" aria-hidden="true">
            <Server :size="22" />
          </span>
          <div>
            <p class="deployment__eyebrow">Server / Docker</p>
            <h4 id="storage-server-title">一个存储位置，可挂载多个资源库</h4>
          </div>
        </header>

        <ol class="storage-flow" aria-label="Server 存储路径">
          <li class="storage-flow__step">
            <HardDrive :size="18" aria-hidden="true" />
            <div>
              <strong><code>./lumilio/media</code></strong>
              <span>宿主机或 NAS 上的媒体目录</span>
            </div>
          </li>
          <li class="storage-flow__arrow" aria-hidden="true">
            <ArrowDown :size="16" />
            <span>Compose 挂载</span>
          </li>
          <li class="storage-flow__step">
            <Container :size="18" aria-hidden="true" />
            <div>
              <strong><code>/data/storage</code></strong>
              <span>容器内唯一的存储位置，由 Compose 预先挂载</span>
            </div>
          </li>
          <li class="storage-flow__arrow" aria-hidden="true">
            <ArrowDown :size="16" />
            <span>创建或登记资源库</span>
          </li>
          <li class="storage-flow__step storage-flow__step--result">
            <FolderCheck :size="18" aria-hidden="true" />
            <div>
              <strong><code>/data/storage/&lt;资源库目录&gt;</code></strong>
              <span>名称原样成为目录；每个子目录都可以是独立磁盘挂载点</span>
            </div>
          </li>
        </ol>

        <dl class="deployment__facts">
          <div>
            <dt>存储位置</dt>
            <dd>整个 Server 实例只有 <code>/data/storage</code> 这一个位置。</dd>
          </div>
          <div>
            <dt>资源库</dt>
            <dd>可以创建多个 <code>.lumiliorepo</code>；无法添加第二个 <code>.lumilioroot</code>。</dd>
          </div>
        </dl>
      </article>
    </div>

    <div class="storage-comparison__state">
      <Database :size="18" aria-hidden="true" />
      <p>
        <strong>媒体与应用状态必须分开：</strong>
        Server 默认把数据库、密钥和备份放在
        <code>./lumilio/app-state</code>，不要把它挂载到媒体目录中。
      </p>
    </div>
  </div>
</template>

<style scoped>
.storage-comparison {
  margin: 20px 0 24px;
}

.storage-comparison__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.deployment {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--vp-c-divider);
  border-top-width: 3px;
  border-radius: 12px;
  background: var(--vp-c-bg-soft);
}

.deployment--desktop {
  border-top-color: var(--vp-c-brand-1);
}

.deployment--server {
  border-top-color: var(--vp-c-green-1);
}

.deployment__header {
  display: flex;
  align-items: center;
  gap: 11px;
  padding-bottom: 15px;
  border-bottom: 1px solid var(--vp-c-divider);
}

.deployment__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  border-radius: 10px;
}

.deployment--desktop .deployment__icon {
  color: var(--vp-c-brand-1);
  background: var(--vp-c-brand-soft);
}

.deployment--server .deployment__icon {
  color: var(--vp-c-green-1);
  background: var(--vp-c-green-soft);
}

.deployment__eyebrow {
  margin: 0 0 2px;
  color: var(--vp-c-text-3);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  line-height: 1.4;
  text-transform: uppercase;
}

.deployment__header h4 {
  margin: 0;
  border: 0;
  color: var(--vp-c-text-1);
  font-size: 15px;
  line-height: 1.45;
}

.storage-flow {
  margin: 16px 0;
  padding: 0;
  list-style: none;
}

.storage-flow__step {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 11px 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg);
}

.storage-flow__step > svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--vp-c-text-2);
}

.storage-flow__step--result {
  border-color: var(--vp-c-brand-1);
}

.deployment--server .storage-flow__step--result {
  border-color: var(--vp-c-green-1);
}

.storage-flow__step div {
  min-width: 0;
}

.storage-flow__step strong,
.storage-flow__step span {
  display: block;
}

.storage-flow__step strong {
  overflow-wrap: anywhere;
  color: var(--vp-c-text-1);
  font-size: 13px;
  line-height: 1.5;
}

.storage-flow__step span {
  margin-top: 2px;
  color: var(--vp-c-text-3);
  font-size: 12px;
  line-height: 1.55;
}

.storage-flow__step code,
.storage-comparison__state code {
  font-size: 0.92em;
}

.storage-flow__arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1px;
  padding: 5px 0;
  color: var(--vp-c-text-3);
  font-size: 11px;
  line-height: 1.35;
}

.deployment__facts {
  margin: 0;
  padding-top: 14px;
  border-top: 1px solid var(--vp-c-divider);
}

.deployment__facts > div {
  display: grid;
  grid-template-columns: 68px minmax(0, 1fr);
  gap: 8px;
}

.deployment__facts > div + div {
  margin-top: 8px;
}

.deployment__facts dt,
.deployment__facts dd {
  margin: 0;
  font-size: 12px;
  line-height: 1.55;
}

.deployment__facts dt {
  color: var(--vp-c-text-1);
  font-weight: 650;
}

.deployment__facts dd {
  color: var(--vp-c-text-2);
}

.storage-comparison__state {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 9px;
  color: var(--vp-c-text-2);
  background: var(--vp-c-bg-alt);
}

.storage-comparison__state > svg {
  flex: 0 0 auto;
  margin-top: 2px;
  color: var(--vp-c-warning-1);
}

.storage-comparison__state p {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.6;
}

.storage-comparison__state strong {
  color: var(--vp-c-text-1);
}

@media (max-width: 760px) {
  .storage-comparison__grid {
    grid-template-columns: 1fr;
  }

  .deployment {
    padding: 16px;
  }
}
</style>
