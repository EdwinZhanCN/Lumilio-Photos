/**
 * # Lumilio
 *
 * Lumilio owns the authenticated assistant experience: the `/lumilio` board,
 * reusable chat dock, streamed assistant blocks, contextual asset handoff,
 * mentions, slash modes, and durable board pins. Media, collections, people,
 * and settings remain in their own features and contribute only explicit
 * context or public data.
 *
 * ## State
 *
 * {@link useLumilioChatStore} owns thread/run identity, streamed blocks,
 * generation and error state, confirmation interrupts, token usage, and
 * send/resume/stop/new-conversation commands. {@link useContextStore} is the
 * lower shared context bus for current selections and viewer context.
 * {@link useDockStore} owns only the user's collapse override.
 *
 * Pins, ref hydration, widget data, mention sources, and capabilities remain
 * TanStack Query server state. Closing or collapsing the dock does not cancel a
 * run. {@link resetLumilioSession} is the user-session boundary: it starts a
 * best-effort server cancellation, closes transport, and clears chat and
 * context state.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTE["/lumilio"] --> BOARD["AgentBoard"]
 *     ROUTE --> DOCK["embedded ChatDock"]
 *     APP["asset/viewer surfaces"] --> LAUNCHER["AgentDockLauncher"]
 *     LAUNCHER --> DOCK
 *     DOCK --> INPUT["MentionInput + ContextChips"]
 *     INPUT --> STORE["useLumilioChatStore"]
 *     STORE --> SSE["agent stream"]
 *     SSE --> WIDGET["inline widget"]
 *     WIDGET --> PIN["durable pin"]
 *     PIN --> BOARD
 * ```
 *
 * {@link LumilioChatPage} composes {@link AgentBoard} and an embedded
 * {@link ChatDock}. Other routes mount the dock through
 * {@link AgentDockLauncher}. Asset-owned contributors publish selected or
 * viewed media through {@link useBrowseSelectionContext} and
 * {@link useViewerContextContributor}; {@link ContextChips} lets the user
 * exclude a contribution for one send.
 *
 * {@link MentionInput} combines text, typed mentions, slash modes, send,
 * confirmation, and Stop. Inline results render through
 * {@link InlineWidgetCard}; pinning copies the server snapshot and
 * {@link BoardTile} renders the durable result through the widget registry.
 *
 * ## Data
 *
 * {@link streamAgent} opens authenticated SSE for chat/resume.
 * `session_info` binds a run, text becomes {@link TextBlock}, tool/widget side
 * channels become {@link ToolBlock}/{@link WidgetBlock}, and interrupts become
 * {@link ConfirmBlock}. {@link cancelAgentRun} targets the exact thread/run;
 * {@link cancelActiveBlocks} preserves partial text while marking unfinished
 * blocks stopped.
 *
 * {@link createMentionSources} builds typed person, album, pin, camera, and
 * lens constraints. Ref events carry handles rather than full media payloads;
 * {@link useWidgetData} hydrates ref or pin metadata and widget assets remain
 * separate queries. The root `index.ts` exports only the dock surfaces and
 * session reset; stream, widget, and store internals are private.
 *
 * @module
 */
import type { cancelAgentRun, streamAgent } from "./api/agentStream.ts";
import type { AgentBoard } from "./flows/board/AgentBoard.tsx";
import type { AgentDockLauncher } from "./flows/chat/AgentDockLauncher.tsx";
import type { ChatDock } from "./flows/chat/ChatDock.tsx";
import type { ContextChips } from "./flows/chat/ContextChips.tsx";
import type { MentionInput } from "./flows/chat/MentionInput.tsx";
import type LumilioChatPage from "./flows/workspace/LumilioWorkspaceFlow.tsx";
import type { ConfirmBlock, TextBlock, ToolBlock, WidgetBlock } from "./model/chatTypes.ts";
import type { createMentionSources } from "./modules/mentions/mentionSources.ts";
import type { BoardTile } from "./modules/widgets/chrome/BoardTile.tsx";
import type { InlineWidgetCard } from "./modules/widgets/chrome/InlineWidgetCard.tsx";
import type { useWidgetData } from "./modules/widgets/useWidgetData.ts";
import type { cancelActiveBlocks } from "./state/blocks.ts";
import type { useLumilioChatStore } from "./state/chatStore.ts";
import type { resetLumilioSession } from "./state/resetSession.ts";
import type { useBrowseSelectionContext } from "../assets/flows/browse/useBrowseSelectionContext.ts";
import type { useViewerContextContributor } from "../assets/flows/viewer/useViewerContextContributor.ts";
import type { useContextStore, useDockStore } from "../../lib/assistant/index.ts";

export {};
