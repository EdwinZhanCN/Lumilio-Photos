<script setup lang="ts">
import type { Component } from "vue";
import {
  HardDrive,
  Smartphone,
  Cloud,
  ScanLine,
  Upload,
  CloudDownload,
  FolderCheck,
  Inbox,
  ArrowDown,
  Wifi,
  WifiOff,
  TriangleAlert,
  BadgeCheck,
} from "@lucide/vue";

interface Scenario {
  badge: string;
  tone: "recommend" | "info" | "warning";
  sourceIcon: Component;
  sourceLabel: string;
  methodIcon: Component;
  methodLabel: string;
  destIcon: Component;
  destLabel: string;
  destNote: string;
  tags: { icon?: Component; text: string }[];
}

const scenarios: Scenario[] = [
  {
    badge: "推荐",
    tone: "recommend",
    sourceIcon: HardDrive,
    sourceLabel: "本机磁盘 · SD 卡 · USB",
    methodIcon: ScanLine,
    methodLabel: "拷贝/移动到自由区 → 扫描",
    destIcon: FolderCheck,
    destLabel: "自由区",
    destNote: "原地登记，文件不被移动",
    tags: [
      { icon: WifiOff, text: "不依赖网络" },
      { text: "大批量友好" },
      { text: "存储策略不影响" },
    ],
  },
  {
    badge: "依赖网络",
    tone: "info",
    sourceIcon: Smartphone,
    sourceLabel: "手机 · 其他设备",
    methodIcon: Upload,
    methodLabel: "浏览器上传",
    destIcon: Inbox,
    destLabel: "inbox/",
    destNote: "Lumilio 管道归档",
    tags: [
      { icon: Wifi, text: "依赖网络" },
      { text: "少量 / 跨设备" },
      { text: "按存储策略归档" },
    ],
  },
  {
    badge: "实验性",
    tone: "warning",
    sourceIcon: Cloud,
    sourceLabel: "iCloud",
    methodIcon: CloudDownload,
    methodLabel: "云导入",
    destIcon: Inbox,
    destLabel: "inbox/",
    destNote: "Lumilio 管道归档",
    tags: [
      { icon: Wifi, text: "依赖外部服务" },
      { icon: TriangleAlert, text: "永久实验性" },
      { text: "按存储策略归档" },
    ],
  },
];
</script>

<template>
  <div class="import-scenarios">
    <div
      v-for="(s, i) in scenarios"
      :key="i"
      class="scenario"
      :class="`scenario--${s.tone}`"
    >
      <span class="scenario__badge">
        <BadgeCheck v-if="s.tone === 'recommend'" :size="13" />
        {{ s.badge }}
      </span>

      <div class="scenario__source">
        <component :is="s.sourceIcon" :size="22" class="scenario__source-icon" />
        <span class="scenario__source-label">{{ s.sourceLabel }}</span>
      </div>

      <div class="scenario__arrow">
        <ArrowDown :size="16" class="scenario__arrow-icon" />
        <span class="scenario__method">{{ s.methodLabel }}</span>
      </div>

      <div class="scenario__dest">
        <component :is="s.destIcon" :size="22" class="scenario__dest-icon" />
        <div class="scenario__dest-text">
          <span class="scenario__dest-label">{{ s.destLabel }}</span>
          <span class="scenario__dest-note">{{ s.destNote }}</span>
        </div>
      </div>

      <div class="scenario__tags">
        <span
          v-for="tag in s.tags"
          :key="tag.text"
          class="scenario__tag"
        >
          <component v-if="tag.icon" :is="tag.icon" :size="11" />
          {{ tag.text }}
        </span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.import-scenarios {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
  margin: 16px 0;
}

@media (max-width: 768px) {
  .import-scenarios {
    grid-template-columns: 1fr;
  }
}

.scenario {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 16px 14px 12px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  background: var(--vp-c-bg-soft);
  border-top-width: 3px;
}

.scenario--recommend {
  border-top-color: var(--vp-c-green-1);
}
.scenario--recommend .scenario__badge {
  color: var(--vp-c-green-1);
}
.scenario--recommend .scenario__source-icon {
  color: var(--vp-c-green-1);
}
.scenario--recommend .scenario__dest-icon {
  color: var(--vp-c-green-1);
}

.scenario--info {
  border-top-color: var(--vp-c-brand-1);
}
.scenario--info .scenario__badge {
  color: var(--vp-c-brand-1);
}
.scenario--info .scenario__source-icon {
  color: var(--vp-c-brand-1);
}
.scenario--info .scenario__dest-icon {
  color: var(--vp-c-brand-1);
}

.scenario--warning {
  border-top-color: var(--vp-c-warning-1);
}
.scenario--warning .scenario__badge {
  color: var(--vp-c-warning-1);
}
.scenario--warning .scenario__source-icon {
  color: var(--vp-c-warning-1);
}
.scenario--warning .scenario__dest-icon {
  color: var(--vp-c-warning-1);
}

.scenario__badge {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 14px;
}

.scenario__source {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.scenario__source-icon {
  color: var(--vp-c-text-2);
}

.scenario__source-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--vp-c-text-1);
}

.scenario__arrow {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
  padding: 10px 0;
  color: var(--vp-c-text-3);
}

.scenario__arrow-icon {
  opacity: 0.5;
}

.scenario__method {
  font-size: 11.5px;
  color: var(--vp-c-text-3);
}

.scenario__dest {
  display: flex;
  align-items: center;
  gap: 8px;
}

.scenario__dest-icon {
  color: var(--vp-c-text-2);
  flex-shrink: 0;
}

.scenario__dest-text {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
}

.scenario__dest-label {
  font-size: 13px;
  font-weight: 500;
  color: var(--vp-c-text-1);
}

.scenario__dest-note {
  font-size: 11.5px;
  color: var(--vp-c-text-3);
}

.scenario__tags {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 6px;
  margin-top: 14px;
  padding-top: 10px;
  border-top: 1px solid var(--vp-c-divider);
  width: 100%;
}

.scenario__tag {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 10px;
  border: 1px solid var(--vp-c-divider);
  color: var(--vp-c-text-2);
}
</style>
