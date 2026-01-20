// Lumilio-Photos/web/src/features/lumilio/rich-input/index.ts

// ==================== Types ====================
export type {
  MentionType,
  TriggerPhase,
  MentionEntity,
  MenuPosition,
  RichInputState,
  RichInputAction,
  RichInputContextValue,
} from "./types";

export { initialState } from "./types";

// ==================== Provider & Hook ====================
export { RichInputProvider, useRichInput } from "./RichInputProvider";

// ==================== Component ====================
export { RichInput } from "./RichInput";
export type { RichInputProps } from "./RichInput";

// ==================== Utils ====================
export {
  parseContentToPayload,
  createPillElement,
  calculateMenuPosition,
  findLastTriggerChar,
  insertPill,
  clearEditor,
} from "./utils";
