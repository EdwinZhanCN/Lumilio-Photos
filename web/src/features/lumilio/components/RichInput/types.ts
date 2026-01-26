import React from "react";

/** 可提及的类型 */
export type MentionType = "placeholder";

/** 输入触发阶段 */
export type TriggerPhase = "IDLE" | "SELECT_TYPE" | "SELECT_ENTITY" | "COMMAND";

/** 提及实体或命令 */
export interface MentionEntity {
  id: string;
  label: string;
  type: MentionType | "command";
  meta?: string;
  icon?: React.ReactNode;
  desc?: string;
}

/** 提及类型选项（用于菜单显示，不需要 id） */
export interface MentionTypeOption {
  type: MentionType;
  label: string;
  icon?: React.ReactNode;
  desc?: string;
}

/** 菜单位置 */
export interface MenuPosition {
  top: number;
  left: number;
}

// ==================== RichInput 状态 ====================

/** RichInput 状态接口 */
export interface RichInputState {
  /** 当前触发阶段 */
  phase: TriggerPhase;

  /** 当前选中的提及类型（用户选择了 @album 后，这里就是 "album"） */
  activeMentionType: MentionType | null;

  /** 悬浮菜单位置 */
  menuPos: MenuPosition | null;

  /** 当前菜单选项索引（用于键盘导航） */
  selectedIndex: number;

  /** 当前可用的菜单选项列表 */
  options: MentionEntity[];

  /** 解析后的 payload（从 contentEditable 中提取的结构化数据） */
  payload: string;
}

// ==================== Actions 类型 ====================

/** 设置触发阶段 */
export type SetPhaseAction = { type: "SET_PHASE"; payload: TriggerPhase };

/** 设置激活的提及类型 */
export type SetActiveMentionTypeAction = {
  type: "SET_ACTIVE_MENTION_TYPE";
  payload: MentionType | null;
};

/** 设置菜单位置 */
export type SetMenuPositionAction = {
  type: "SET_MENU_POSITION";
  payload: MenuPosition | null;
};

/** 设置选中索引 */
export type SetSelectedIndexAction = {
  type: "SET_SELECTED_INDEX";
  payload: number;
};

/** 设置菜单选项 */
export type SetOptionsAction = {
  type: "SET_OPTIONS";
  payload: MentionEntity[];
};

/** 设置 payload */
export type SetPayloadAction = {
  type: "SET_PAYLOAD";
  payload: string;
};

/** 重置编辑器状态 */
export type ResetEditorAction = { type: "RESET_EDITOR" };

/** 联合所有 action 类型 */
export type RichInputAction =
  | SetPhaseAction
  | SetActiveMentionTypeAction
  | SetMenuPositionAction
  | SetSelectedIndexAction
  | SetOptionsAction
  | SetPayloadAction
  | ResetEditorAction;

// ==================== Context 接口 ====================

export interface RichInputContextValue {
  state: RichInputState;
  dispatch: React.Dispatch<RichInputAction>;
}

// ==================== 初始状态 ====================

export const initialState: RichInputState = {
  phase: "IDLE",
  activeMentionType: null,
  menuPos: null,
  selectedIndex: 0,
  options: [],
  payload: "",
};
