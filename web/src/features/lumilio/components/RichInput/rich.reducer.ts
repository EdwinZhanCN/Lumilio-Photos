import { RichInputState, RichInputAction, initialState } from "./types";

/**
 * RichInput 主 Reducer
 *
 * 管理 RichInput 组件的所有状态，包括：
 * - 触发阶段（phase）
 * - 当前激活的提及类型
 * - 悬浮菜单位置
 * - 选中索引
 * - 菜单选项
 * - 解析后的 payload
 */
export const RichInputReducer = (
  state: RichInputState,
  action: RichInputAction,
): RichInputState => {
  switch (action.type) {
    // ========== 阶段管理 ==========
    case "SET_PHASE":
      return {
        ...state,
        phase: action.payload,
      };

    // ========== 提及类型管理 ==========
    case "SET_ACTIVE_MENTION_TYPE":
      return {
        ...state,
        activeMentionType: action.payload,
      };

    // ========== 菜单位置管理 ==========
    case "SET_MENU_POSITION":
      return {
        ...state,
        menuPos: action.payload,
      };

    // ========== 选中索引管理 ==========
    case "SET_SELECTED_INDEX":
      return {
        ...state,
        selectedIndex: action.payload,
      };

    // ========== 菜单选项管理 ==========
    case "SET_OPTIONS":
      return {
        ...state,
        options: action.payload,
      };

    // ========== Payload 管理 ==========
    case "SET_PAYLOAD":
      return {
        ...state,
        payload: action.payload,
      };

    // ========== 重置编辑器 ==========
    case "RESET_EDITOR":
      return {
        ...initialState,
      };

    default:
      return state;
  }
};

export type { RichInputAction };
export { initialState };
