<script setup lang="ts">
import { ref, type Component } from "vue";
import {
  ChevronRight,
  Folder,
  FolderOpen,
  File,
  Image,
  Film,
  Music,
  KeyRound,
  Package,
  Inbox,
  Aperture,
  Trash2,
  Edit3,
  FileText,
} from "@lucide/vue";

defineOptions({ name: "FileTree" });

export interface TreeNode {
  name: string;
  kind: "folder" | "file";
  icon?: "marker" | "system" | "inbox" | "trash" | "sidecar" | "log";
  annotation?: string;
  hidden?: boolean;
  expanded?: boolean;
  children?: TreeNode[];
}

const props = withDefaults(
  defineProps<{
    nodes: TreeNode[];
    level?: number;
  }>(),
  { level: 0 },
);

const expanded = ref<Record<number, boolean>>({});

props.nodes.forEach((node, i) => {
  if (node.children?.length && (node.expanded ?? false)) {
    expanded.value[i] = true;
  }
});

function toggle(i: number) {
  expanded.value[i] = !expanded.value[i];
}

const specialIcons: Record<string, Component> = {
  marker: KeyRound,
  system: Package,
  inbox: Inbox,
  trash: Trash2,
  sidecar: Edit3,
  log: FileText,
};

const extIcons: Record<string, Component> = {
  jpg: Image, jpeg: Image, png: Image, webp: Image, gif: Image,
  bmp: Image, tiff: Image, tif: Image, heic: Image, heif: Image,
  mp4: Film, mov: Film, avi: Film, mkv: Film, webm: Film,
  mp3: Music, aac: Music, m4a: Music, flac: Music, wav: Music, ogg: Music,
  cr2: Aperture, cr3: Aperture, nef: Aperture, arw: Aperture, dng: Aperture,
  orf: Aperture, rw2: Aperture, pef: Aperture, raf: Aperture,
  mrw: Aperture, srw: Aperture, rwl: Aperture, x3f: Aperture,
};

function getIcon(node: TreeNode, isOpen: boolean): Component {
  if (node.kind === "folder") return isOpen ? FolderOpen : Folder;
  if (node.icon && specialIcons[node.icon]) return specialIcons[node.icon];
  const ext = node.name.split(".").pop()?.toLowerCase();
  if (ext && extIcons[ext]) return extIcons[ext];
  return File;
}

const hasChildren = (node: TreeNode) => !!node.children?.length;
const isOpen = (i: number) => expanded.value[i] ?? false;
</script>

<template>
  <div class="file-tree">
    <div
      v-for="(node, i) in nodes"
      :key="i"
      class="file-tree__node"
    >
      <div
        class="file-tree__row"
        :class="{
          'is-folder': node.kind === 'folder',
          'is-hidden': node.hidden,
        }"
        :style="{ paddingLeft: `${level * 18 + 2}px` }"
        :role="hasChildren(node) ? 'button' : undefined"
        :tabindex="hasChildren(node) ? 0 : undefined"
        @click="hasChildren(node) && toggle(i)"
        @keydown.enter.prevent="hasChildren(node) && toggle(i)"
      >
        <span
          v-if="hasChildren(node)"
          class="file-tree__chevron"
          :class="{ 'file-tree__chevron--open': isOpen(i) }"
        >
          <ChevronRight :size="13" />
        </span>
        <span v-else class="file-tree__chevron-placeholder" />

        <component
          :is="getIcon(node, isOpen(i))"
          :size="15"
          class="file-tree__icon"
          aria-hidden="true"
        />

        <span class="file-tree__name">{{ node.name }}</span>

        <span v-if="node.annotation" class="file-tree__annotation">
          {{ node.annotation }}
        </span>
      </div>

      <div
        v-if="hasChildren(node) && isOpen(i)"
        class="file-tree__children"
      >
        <FileTree :nodes="node.children!" :level="level + 1" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.file-tree {
  font-family: var(--vp-font-family-mono);
  font-size: 13px;
  line-height: 1.7;
}

.file-tree__row {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 1px 8px 1px 2px;
  border-radius: 4px;
  cursor: default;
  white-space: nowrap;
  transition: background 0.12s;
}

.file-tree__row.is-folder {
  cursor: pointer;
}

.file-tree__row.is-folder:hover {
  background: var(--vp-c-bg-soft);
}

.file-tree__row.is-hidden {
  opacity: 0.6;
}

.file-tree__chevron {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 13px;
  height: 13px;
  flex-shrink: 0;
  color: var(--vp-c-text-3);
  transition: transform 0.15s ease;
}

.file-tree__chevron--open {
  transform: rotate(90deg);
}

.file-tree__chevron-placeholder {
  width: 13px;
  flex-shrink: 0;
}

.file-tree__icon {
  flex-shrink: 0;
  color: var(--vp-c-text-2);
}

.file-tree__row.is-folder .file-tree__icon {
  color: var(--vp-c-brand-1);
}

.file-tree__name {
  color: var(--vp-c-text-1);
}

.file-tree__annotation {
  margin-left: 12px;
  color: var(--vp-c-text-3);
  font-family: var(--vp-font-family-base);
  font-size: 12.5px;
}

@media (max-width: 768px) {
  .file-tree__annotation {
    display: none;
  }
}
</style>
