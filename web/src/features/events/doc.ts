/**
 * # Events
 *
 * Events owns deterministic, user-correctable media organization.
 *
 * ## State
 *
 * Server Event state remains in TanStack Query through {@link useEvents} and
 * {@link useEvent}. Gallery selection remains owned by the Assets scope.
 *
 * ## Flows
 *
 * ```mermaid
 * flowchart TD
 *     ROUTES["Event routes"] --> FLOWS["Index and detail flows"]
 *     FLOWS --> BROWSER["Assets public entry"]
 *     FLOWS --> API["Event query hooks"]
 * ```
 *
 * {@link EventsIndexFlow} presents stable Event summaries and cursor paging.
 * {@link EventDetailFlow} composes the public Assets browser with an
 * owner-scoped `event_id` constraint. {@link EventHero} owns the collection
 * presentation and entry points into focused edit, share, and add-media
 * dialogs. {@link EventEditModal} keeps metadata and merge correction together,
 * using {@link EventPicker} instead of exposing internal Event IDs.
 * {@link useEventBulkActions} owns selection-derived cover, split, move,
 * remove, and snapshot-share workflows outside the route component.
 *
 * ## Data
 *
 * {@link EventSummary} is generated from OpenAPI. Event titles are live,
 * localized presentation fallbacks rather than persisted prose.
 *
 * @module
 */
import type { useEvent, useEvents } from "./api/useEvents.ts";
import type { EventDetailFlow } from "./flows/detail/EventDetailFlow.tsx";
import type EventEditModal from "./flows/detail/components/EventEditModal.tsx";
import type EventHero from "./flows/detail/components/EventHero.tsx";
import type EventPicker from "./flows/detail/components/EventPicker.tsx";
import type { useEventBulkActions } from "./flows/detail/useEventBulkActions.tsx";
import type { EventsIndexFlow } from "./flows/index/EventsIndexFlow.tsx";
import type { EventSummary } from "./model/event.ts";

export {};
