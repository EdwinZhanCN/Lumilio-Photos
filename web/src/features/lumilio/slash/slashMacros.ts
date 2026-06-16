/** Slash macro definitions — expand to visible user messages before send. */

export interface SlashMacro {
  id: string;
  label: string;
  description: string;
  /** Template with optional bracketed placeholders the user can edit. */
  template: string;
}

export const SLASH_MACROS: SlashMacro[] = [
  {
    id: "review",
    label: "回顾",
    description: "Filter, rank by quality, show top picks",
    template: "回顾最近一个月的照片，按画质挑选最好的 20 张展示",
  },
  {
    id: "on-this-day",
    label: "那年今天",
    description: "Photos from this calendar day across years",
    template: "找出历年今天前后一周拍的照片，按时间排序展示",
  },
  {
    id: "curate",
    label: "精选",
    description: "Rank by quality and suggest an album",
    template: "从最近上传的照片里精选 12 张画质最好的，做成相册",
  },
  {
    id: "gear-stats",
    label: "器材统计",
    description: "Describe camera/lens distribution",
    template: "统计我库里各相机的拍摄数量分布",
  },
];

export function expandSlashCommand(label: string): string | null {
  const macro = SLASH_MACROS.find((m) => m.label === label || m.id === label);
  return macro?.template ?? null;
}

export const QUICK_ASKS = [
  "上月新增了多少张照片？要回顾吗？",
  "帮我找出最近一周拍的竖图",
  "精选最近上传里画质最好的 9 张",
];
