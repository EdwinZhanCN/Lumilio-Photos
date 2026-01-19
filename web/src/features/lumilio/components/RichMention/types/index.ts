export type MentionType = "album" | "tag" | "camera" | "lens" | "location";
export type TriggerPhase = "IDLE" | "SELECT_TYPE" | "SELECT_ENTITY" | "COMMAND";

export interface MentionEntity {
  id: string;
  label: string;
  type: MentionType | "command";
  meta?: string;
  icon?: React.ReactNode;
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
  command?: {
    name: string;
    params?: Record<string, any>;
  };
  commandPayload?: any;
}

export interface MenuPosition {
  top: number;
  left: number;
}
