/** Represents a side-channel event for tool execution status updates.
 * These events provide real-time feedback about tool execution progress,
 * including status changes, errors, and output data.
 */
export interface SideChannelEvent {
  type: string;
  timestamp: number;
  tool: ToolIdentity;
  execution: ExecutionInfo;
  data?: DataPayload;
  extra?: ExtraInfo;
}

/** Represents a message event in the agent's main event stream.
 * Contains the agent's response, reasoning, potential actions, and any errors
 * encountered during execution.
 */
export interface AgentMessageEvent {
  agent_name: string;
  run_path?: string[];
  output?: string;
  reasoning?: string;
  action?: AgentAction;
  error?: string;
}

// --- SSE 事件包装 ---

/** Represents a Server-Sent Event (SSE) wrapper.
 * Generic type T determines the payload type for different event types.
 * Events include session info, messages, UI events, actions, completion,
 * errors, and heartbeat signals.
 */
export interface SseEvent<T> {
  type:
    | "session_info"
    | "message"
    | "ui_event"
    | "action"
    | "done"
    | "error"
    | "heartbeat";
  data: T;
}

/** Session information provided when a connection is established.
 * Contains the unique thread identifier for the current conversation session.
 */
export interface SessionInfoEvent {
  thread_id: string;
}

// --- 核心数据结构 ---

/** Identifies a tool execution instance.
 * Contains the tool name and a unique execution identifier for tracking
 * specific invocations throughout the conversation.
 */
export interface ToolIdentity {
  name: string;
  executionId: string;
}

/** Provides detailed information about a tool's execution status.
 * Tracks the lifecycle of tool execution including status, error details,
 * execution parameters, and duration metrics.
 */
export interface ExecutionInfo {
  status: "pending" | "running" | "success" | "error" | "cancelled";
  message?: string;
  error?: ErrorInfo;
  parameters?: any;
  duration?: number;
}

/** Structured error information for failed operations.
 * Provides error code, human-readable message, and additional details
 * for debugging and user feedback.
 */
export interface ErrorInfo {
  code: string;
  message: string;
  details?: any;
}

/** Represents structured data payload returned by tools.
 * Contains a reference ID, payload type identifier, the actual data (type
 * varies by payload_type), and optional rendering configuration for display.
 */
export interface DataPayload {
  refId: string;
  payload_type: string;
  payload: any;
  rendering?: RenderingConfig;
}

/** Configuration for rendering tool output data.
 * Specifies the component type to use for display and any additional
 * configuration options for that component.
 */
export interface RenderingConfig {
  component: "justified_gallery" | "data_table" | "chart";
  config?: any;
}

/** Additional metadata or context information for events.
 * Provides extensible key-value pairs for storing supplementary information
 * not covered by other fields.
 */
export interface ExtraInfo {
  extra_type: string;
  data: any;
}

// --- Agent Action 和中断 ---

/** Represents actions that an agent may take during execution.
 * Currently supports interruption information, extensible for future action types.
 */
export interface AgentAction {
  interrupted?: InterruptInfo;
}

/** Information about an interruption in agent execution.
 * Contains the confirmation data requiring user action and context about
 * where and why the interruption occurred.
 */
export interface InterruptInfo {
  data: FilterConfirmationInfo;
  InterruptContexts: InterruptContext[];
}

/** Context information for an interruption event.
 * Provides unique identifier, execution path address, confirmation info,
 * and whether this is the root cause of the interruption.
 */
export interface InterruptContext {
  ID: string;
  Address: any[];
  Info: FilterConfirmationInfo;
  IsRootCause: boolean;
}

/** Information sent to the frontend during an interruption for confirmation.
 * Contains details about what requires user confirmation, including the
 * affected item count, unique confirmation ID, and descriptive message.
 */
export interface FilterConfirmationInfo {
  count: number;
  confirmationId: string;
  message: string;
}
