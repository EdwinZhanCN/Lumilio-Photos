import { useI18n } from "@/lib/i18n";
import type { ReactNode } from "react";
import { CSS } from "@dnd-kit/utilities";
import {
  DndContext,
  closestCenter,
  PointerSensor,
  KeyboardSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
  sortableKeyboardCoordinates,
} from "@dnd-kit/sortable";
import { GripVertical } from "lucide-react";
import MusicTrackRow from "./MusicTrackRow";
import type { MusicPlaybackSource, MusicTrack } from "../model/music";

type MusicTrackListProps = {
  tracks: MusicTrack[];
  source: MusicPlaybackSource;
  /** Map a track to the playlist entry id (used to highlight the currently-playing entry). */
  getEntryId?: (track: MusicTrack, index: number) => string | undefined;
  /** Offset applied to the on-screen index (e.g. artist pagination). */
  indexOffset?: number;
  /** Show the album-name column (default true). */
  showAlbumColumn?: boolean;
  /** Text shown when `tracks` is empty (callers may handle empty/loading/error themselves). */
  emptyText?: string;
  /** Called with the list index when a row's Remove action is triggered. */
  onRemove?: (index: number) => void;
  /** Per-row trailing controls (rendered after the row, inside the row wrapper). */
  rowActions?: (track: MusicTrack, index: number) => ReactNode;
  /** Enable drag-to-reorder. `getKey` maps a row to a stable sortable id; `onReorder` is called on drop. */
  sortable?: {
    getKey: (track: MusicTrack, index: number) => string;
    onReorder: (activeId: string, overId: string) => void;
  };
};

function SortableTrackRow({ id, children }: { id: string; children: ReactNode }) {
  const { t } = useI18n();
  const { attributes, listeners, setNodeRef, transform, transition } = useSortable({ id });
  return (
    <div
      ref={setNodeRef}
      style={{ transform: CSS.Transform.toString(transform), transition }}
      className="flex items-center gap-1"
    >
      <button
        type="button"
        className="btn btn-ghost btn-xs btn-square cursor-grab opacity-60 group-hover:opacity-100"
        aria-label={t("music.playlist.dragReorder", "Drag to reorder")}
        {...attributes}
        {...listeners}
      >
        <GripVertical className="size-4" />
      </button>
      {children}
    </div>
  );
}

/**
 * Shared Tracks-style list: `divide-y` rows rendered with the browse Tracks
 * appearance (artwork + title + artist + album column + like + duration + more).
 * Used by the Tracks browse view and the album / playlist / artist detail views.
 * Pass `sortable` to enable drag-to-reorder (used by the playlist detail).
 */
export default function MusicTrackList({
  tracks,
  source,
  getEntryId,
  indexOffset = 0,
  showAlbumColumn = true,
  emptyText,
  onRemove,
  rowActions,
  sortable,
}: MusicTrackListProps) {
  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );

  if (tracks.length === 0) {
    if (!emptyText) return null;
    return <p className="p-5 text-sm text-base-content/60">{emptyText}</p>;
  }

  const row = (track: MusicTrack, index: number) => (
    <div
      key={getEntryId?.(track, index) ?? track.track_id ?? index}
      className="flex items-center gap-1"
    >
      <div className="min-w-0 flex-1">
        <MusicTrackRow
          track={track}
          entryId={getEntryId?.(track, index)}
          index={indexOffset + index}
          source={source}
          showAlbumColumn={showAlbumColumn}
          onRemove={onRemove ? () => onRemove(index) : undefined}
        />
      </div>
      {rowActions?.(track, index)}
    </div>
  );

  if (!sortable) {
    return <div className="divide-y divide-base-200">{tracks.map(row)}</div>;
  }

  const keys = tracks.map((track, index) => sortable.getKey(track, index));

  const onDragEnd = (event: DragEndEvent) => {
    const { active, over } = event;
    if (over && active.id !== over.id) {
      sortable.onReorder(String(active.id), String(over.id));
    }
  };

  return (
    <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={onDragEnd}>
      <SortableContext items={keys} strategy={verticalListSortingStrategy}>
        <div className="divide-y divide-base-200">
          {tracks.map((track, index) => (
            <SortableTrackRow
              key={getEntryId?.(track, index) ?? track.track_id ?? index}
              id={sortable.getKey(track, index)}
            >
              <div className="min-w-0 flex-1">
                <MusicTrackRow
                  track={track}
                  entryId={getEntryId?.(track, index)}
                  index={indexOffset + index}
                  source={source}
                  showAlbumColumn={showAlbumColumn}
                  onRemove={onRemove ? () => onRemove(index) : undefined}
                />
              </div>
              {rowActions?.(track, index)}
            </SortableTrackRow>
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}
