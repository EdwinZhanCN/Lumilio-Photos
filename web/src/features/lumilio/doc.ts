/**
 * # Lumilio
 *
 * The Lumilio feature owns the authenticated agent experience: the `/lumilio`
 * board route, the reusable chat dock, streamed assistant blocks, contextual
 * asset handoff, `@` mentions, `/` modes, and durable board pins. It is not the
 * base media workflow; assets, collections, people, and settings stay in their
 * own features and Lumilio consumes them through explicit context or mentions.
 *
 * ## State
 *
 * Feature-local interactive state and the shared assistant surface use three
 * small Zustand stores with explicit ownership:
 *
 * - {@link useLumilioChatStore} owns the thread id, active run id, streamed
 *   message blocks, generation/stopping/error state, confirmation interrupts,
 *   token usage, and the send/resume/stop/new-conversation commands. Stop is a
 *   server operation: it calls {@link cancelAgentRun} with the exact
 *   `(thread_id, run_id)` before aborting the local SSE transport.
 * - The lower-level {@link useContextStore} is the cross-surface context bus. Contributors
 *   register current asset selections or carousel viewing context, and
 *   {@link ContextChips} lets the user exclude a contribution before send.
 * - The lower-level {@link useDockStore} owns only the user's chat collapse override; route
 *   defaults still decide whether an untouched dock starts expanded or collapsed.
 *
 * Server state stays in TanStack Query: pins, ref hydration, widget metadata,
 * widget assets, mention source lists, and capabilities are fetched at the
 * component/hook edges instead of being mirrored into those stores.
 *
 * ## Data
 *
 * {@link streamAgent} opens authenticated SSE streams to `/api/v1/agent/chat`
 * and `/api/v1/agent/chat/resume`. Every request sends an explicit mode
 * (`free` for the unconstrained tool set). `session_info` binds the stream to a
 * fresh run id and `run_status` carries {@link AgentRunStatus}. Product SSE
 * contains assistant text only—provider reasoning is never accepted into the
 * public stream contract. Assistant chunks become {@link TextBlock}; tool
 * status and widgets arrive through {@link SideChannelEvent} and become
 * {@link ToolBlock} / {@link WidgetBlock}. An interrupt becomes a
 * {@link ConfirmBlock} and resumes through the same store.
 *
 * {@link cancelActiveBlocks} preserves partial assistant text after Stop,
 * marks unfinished tools `cancelled`, disables an unresolved confirmation, and
 * marks the turn as stopped. {@link MentionInput} reuses its primary action
 * slot as a Stop button while a run is generating or awaiting confirmation.
 * `newConversation` waits for that cancellation request before clearing local
 * state; collapsing or closing the dock never cancels.
 *
 * The stream side channel passes handles, not full asset payloads:
 * {@link RefPayload} carries a ref id, count, widget hint, and params. Inline
 * widgets hydrate that handle through {@link InlineWidgetCard}; durable pins
 * copy the snapshot server-side through {@link PinButton} and are later read by
 * {@link AgentBoard}. {@link useWidgetData} normalizes ref/pin metadata into
 * {@link WidgetData}, while thumbnail-heavy views fetch assets separately.
 *
 * Mentions are explicit, typed constraints. {@link MentionInput} uses
 * {@link createMentionSources} to build searchable person, album, pin, camera,
 * and lens sources; picked entities are sent as {@link MentionPayload}. Slash
 * modes come from {@link useSlashMacros} and constrain the tool subset without
 * inserting a canned prompt.
 *
 * ## Composition
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/lumilio"] --> BOARD["AgentBoard"]
 *     ROUTE --> DOCK["ChatDock embedded"]
 *     FAB["Asset / carousel surfaces"] --> DOCK2["ChatDock fab"]
 *     DOCK --> INPUT["MentionInput"]
 *     DOCK --> CHIPS["ContextChips"]
 *     DOCK --> MESSAGES["ChatMessages"]
 *     INPUT --> STORE["useLumilioChatStore"]
 *     STORE --> CANCEL["cancelAgentRun"]
 *     CHIPS --> CTX["useContextStore"]
 *     GALLERY["useBrowseSelectionContext"] --> CTX
 *     CAROUSEL["useViewerContextContributor"] --> CTX
 *     STORE --> SSE["streamAgent"]
 *     MESSAGES --> INLINE["InlineWidgetCard"]
 *     INLINE --> PIN["PinButton"]
 *     PIN --> BOARD
 *     BOARD --> TILE["BoardTile"]
 *     TILE --> DATA["useWidgetData"]
 *     TILE --> REG["widget registry"]
 * ```
 *
 * {@link LumilioChatPage} is intentionally thin: it renders {@link AgentBoard}
 * and an embedded {@link ChatDock}. The dock composes {@link MentionInput},
 * {@link ContextChips}, and {@link ChatMessages}; asset and carousel surfaces
 * mount it in `fab` mode. Asset-owned contributors publish context through the
 * shared assistant bus via
 * {@link useBrowseSelectionContext} / {@link useViewerContextContributor}.
 * Board pins render through {@link BoardTile}, so the agent UI is a feature
 * overlay rather than another gallery implementation.
 *
 * ## Decisions
 *
 * Context is opt-out at send time. Contributions stay visible as chips, and
 * exclusions are cleared after sending so the next message starts from the
 * current page context rather than a hidden stale exclusion.
 * {@link resetLumilioSession} is the feature reset exposed to application
 * composition. It starts a best-effort server cancellation before closing the
 * transport, then clears the chat store and shared context bus so conversation,
 * contributions, and exclusions never cross a user boundary.
 *
 * Pins are the durability boundary. Chat widgets are session refs; pinning
 * copies the result to `/api/v1/agent/pins`, after which layout, title, view,
 * size, and removal are board concerns.
 *
 * Widget views are registry entries. {@link registerWidget} wires a widget type
 * to its view and icon; all views share the same S/M/L footprints from
 * {@link DIMS}, so switching view never resizes the board cell.
 *
 * @module
 */
import type LumilioChatPage from "./flows/workspace/LumilioWorkspaceFlow.tsx";
import type { AgentBoard } from "./flows/board/AgentBoard.tsx";
import type { ChatDock } from "./flows/chat/ChatDock.tsx";
import type { ChatMessages } from "./flows/chat/ChatMessages.tsx";
import type { ContextChips } from "./flows/chat/ContextChips.tsx";
import type { MentionInput } from "./flows/chat/MentionInput.tsx";
import type { InlineWidgetCard } from "./modules/widgets/chrome/InlineWidgetCard.tsx";
import type { BoardTile } from "./modules/widgets/chrome/BoardTile.tsx";
import type { PinButton } from "./modules/widgets/PinButton.tsx";
import type { cancelAgentRun, streamAgent } from "./api/agentStream.ts";
import type { createMentionSources, MentionPayload } from "./modules/mentions/mentionSources.ts";
import type { useSlashMacros } from "./modules/slash/slashMacros.ts";
import type { useLumilioChatStore } from "./state/chatStore.ts";
import type { cancelActiveBlocks } from "./state/blocks.ts";
import type { useContextStore, useDockStore } from "@/lib/assistant/index.ts";
import type { useBrowseSelectionContext } from "@/features/assets/flows/browse/useBrowseSelectionContext.ts";
import type { useViewerContextContributor } from "@/features/assets/flows/viewer/useViewerContextContributor.ts";
import type { resetLumilioSession } from "./state/resetSession.ts";
import type {
  ConfirmBlock,
  AgentRunStatus,
  RefPayload,
  SideChannelEvent,
  TextBlock,
  ToolBlock,
  WidgetBlock,
} from "./model/chatTypes.ts";
import type { DIMS, registerWidget } from "./modules/widgets/registry.ts";
import type { useWidgetData } from "./modules/widgets/useWidgetData.ts";
import type { WidgetData } from "./modules/widgets/types.ts";

export {};
