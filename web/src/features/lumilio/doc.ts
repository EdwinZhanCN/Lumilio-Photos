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
 * Feature-local interactive state lives in three Zustand stores:
 *
 * - {@link useLumilioChatStore} owns the thread id, streamed message blocks,
 *   generation/error state, confirmation interrupts, token usage, and the
 *   send/resume/new-conversation commands. Its session reset aborts the active
 *   SSE request before clearing the conversation.
 * - {@link useContextStore} is the cross-surface context bus. Contributors
 *   register current asset selections or carousel viewing context, and
 *   {@link ContextChips} lets the user exclude a contribution before send.
 * - {@link useDockStore} owns only the user's chat collapse override; route
 *   defaults still decide whether an untouched dock starts expanded or collapsed.
 *
 * Server state stays in TanStack Query: pins, ref hydration, widget metadata,
 * widget assets, mention source lists, and capabilities are fetched at the
 * component/hook edges instead of being mirrored into those stores.
 *
 * ## Data
 *
 * {@link streamAgent} opens authenticated SSE streams to `/api/v1/agent/chat`
 * and `/api/v1/agent/chat/resume`. The stream emits typed chat blocks:
 * {@link TextBlock}, {@link ReasoningBlock}, {@link ToolBlock},
 * {@link WidgetBlock}, and {@link ConfirmBlock}. Tool status, widgets, and
 * token usage arrive through {@link SideChannelEvent}; an interrupt becomes a
 * confirmation card and resumes through the same store.
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
 *     CHIPS --> CTX["useContextStore"]
 *     GALLERY["useGalleryContextContributor"] --> CTX
 *     CAROUSEL["useCarouselContextContributor"] --> CTX
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
 * mount it in `fab` mode and contribute context through
 * {@link useGalleryContextContributor} / {@link useCarouselContextContributor}.
 * Board pins render through {@link BoardTile}, so the agent UI is a feature
 * overlay rather than another gallery implementation.
 *
 * ## Decisions
 *
 * Context is opt-out at send time. Contributions stay visible as chips, and
 * exclusions are cleared after sending so the next message starts from the
 * current page context rather than a hidden stale exclusion.
 * Both Lumilio stores also expose a full session reset used by authentication;
 * conversation, contributions, and exclusions never cross a user boundary.
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
import type LumilioChatPage from "./routes/LumilioChat.tsx";
import type { AgentBoard } from "./components/Board/AgentBoard.tsx";
import type { ChatDock } from "./components/Chat/ChatDock.tsx";
import type { ChatMessages } from "./components/Chat/ChatMessages.tsx";
import type { ContextChips } from "./components/Chat/ContextChips.tsx";
import type { MentionInput } from "./components/Chat/MentionInput.tsx";
import type { InlineWidgetCard } from "./widgets/chrome/InlineWidgetCard.tsx";
import type { BoardTile } from "./widgets/chrome/BoardTile.tsx";
import type { PinButton } from "./widgets/PinButton.tsx";
import type { streamAgent } from "./api/agentStream.ts";
import type { createMentionSources, MentionPayload } from "./mentions/mentionSources.ts";
import type { useSlashMacros } from "./slash/slashMacros.ts";
import type { useLumilioChatStore } from "./state/chatStore.ts";
import type { useContextStore } from "./state/contextStore.ts";
import type { useDockStore } from "./state/dockStore.ts";
import type { useGalleryContextContributor } from "./contributors/useGalleryContextContributor.ts";
import type { useCarouselContextContributor } from "./contributors/useCarouselContextContributor.ts";
import type {
  ConfirmBlock,
  ReasoningBlock,
  RefPayload,
  SideChannelEvent,
  TextBlock,
  ToolBlock,
  WidgetBlock,
} from "./types.ts";
import type { DIMS, registerWidget } from "./widgets/registry.ts";
import type { useWidgetData } from "./widgets/useWidgetData.ts";
import type { WidgetData } from "./widgets/types.ts";

export {};
