<script setup lang="ts">
import FileTree, { type TreeNode } from "./FileTree.vue";

const tree: TreeNode[] = [
  {
    name: "媒体硬盘/",
    kind: "folder",
    annotation: "存储位置",
    expanded: true,
    children: [
      {
        name: ".lumilioroot",
        kind: "file",
        icon: "marker",
        annotation: "这块存储位置的可移植身份",
        hidden: true,
      },
      {
        name: "primary/",
        kind: "folder",
        annotation: "主资源库",
        expanded: true,
        children: [
          {
            name: ".lumiliorepo",
            kind: "file",
            icon: "marker",
            annotation: "资源库身份与存储规则",
            hidden: true,
          },
          {
            name: "inbox/",
            kind: "folder",
            icon: "inbox",
            annotation: "Lumilio 管道管理，请勿手动操作",
            expanded: true,
            children: [
              {
                name: "2025/",
                kind: "folder",
                children: [
                  {
                    name: "12/",
                    kind: "folder",
                    children: [
                      { name: "IMG_1001.HEIC", kind: "file" },
                      { name: "IMG_1002.MOV", kind: "file" },
                    ],
                  },
                ],
              },
              {
                name: "2026/",
                kind: "folder",
                children: [
                  {
                    name: "01/",
                    kind: "folder",
                    children: [
                      { name: "DSC_2048.RAF", kind: "file" },
                    ],
                  },
                ],
              },
            ],
          },
          {
            name: "已有目录/",
            kind: "folder",
            annotation: "自由区：可自行组织，改动后需重新扫描",
            children: [
              {
                name: "旅行/",
                kind: "folder",
                children: [
                  { name: "海边.jpg", kind: "file" },
                ],
              },
            ],
          },
          {
            name: ".lumilio/",
            kind: "folder",
            icon: "system",
            annotation: "Lumilio 管理的系统目录",
            hidden: true,
            children: [
              {
                name: "assets/",
                kind: "folder",
                annotation: "缩略图、转码与人脸派生文件",
              },
              {
                name: "sidecars/",
                kind: "folder",
                icon: "sidecar",
                annotation: "非破坏性编辑记录",
              },
              {
                name: "staging/",
                kind: "folder",
                annotation: "正在接入与接入失败的文件",
              },
              {
                name: "temp/",
                kind: "folder",
                annotation: "临时处理文件",
              },
              {
                name: "trash/",
                kind: "folder",
                icon: "trash",
                annotation: "尚未永久删除的媒体",
              },
              {
                name: "logs/",
                kind: "folder",
                icon: "log",
                annotation: "该资源库的操作日志",
              },
            ],
          },
        ],
      },
      {
        name: "家庭媒体/",
        kind: "folder",
        annotation: "另一个独立资源库",
        children: [
          {
            name: ".lumiliorepo",
            kind: "file",
            icon: "marker",
            hidden: true,
          },
          {
            name: "inbox/",
            kind: "folder",
            icon: "inbox",
            annotation: "Lumilio 管道管理，请勿手动操作",
          },
          {
            name: ".lumilio/",
            kind: "folder",
            icon: "system",
            hidden: true,
          },
        ],
      },
    ],
  },
];
</script>

<template>
  <div class="repo-tree">
    <FileTree :nodes="tree" />
  </div>
</template>

<style scoped>
.repo-tree {
  margin: 16px 0;
  padding: 12px 8px;
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
  overflow-x: auto;
}
</style>
