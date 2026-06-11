import { useCallback, useMemo, useRef } from "react";
import {
  GridLayout,
  useContainerWidth,
  type Layout,
  type LayoutItem,
} from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import { Pin, Trash2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { getWidget } from "../../widgets/registry";
import type { AgentPinDTO } from "../../widgets/types";
import type { ApiResult } from "../../types";

const BOARD_COLS = 12;
const ROW_HEIGHT = 72;

/** The durable widget board: every cell is a pinned ref rendered through the
 * widget registry, draggable and resizable via react-grid-layout. Layout
 * changes persist to the pins API. */
export function AgentBoard() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { width, containerRef, mounted } = useContainerWidth();

  const pinsQuery = $api.useQuery("get", "/api/v1/agent/pins", {});
  const pins = useMemo(
    () =>
      ((pinsQuery.data as ApiResult<AgentPinDTO[]> | undefined)?.data ?? []),
    [pinsQuery.data],
  );

  const layoutMutation = $api.useMutation("patch", "/api/v1/agent/pins/layout");
  const deleteMutation = $api.useMutation(
    "delete",
    "/api/v1/agent/pins/{id}",
    {
      onSuccess: () =>
        void queryClient.invalidateQueries({
          queryKey: ["get", "/api/v1/agent/pins"],
        }),
    },
  );

  const layout = useMemo<Layout>(
    () =>
      pins.map((pin) => {
        const constraints = getWidget(pin.widget ?? "")?.defaultLayout;
        return {
          i: pin.pin_id!,
          x: pin.layout?.x ?? 0,
          y: pin.layout?.y ?? 0,
          w: pin.layout?.w ?? 4,
          h: pin.layout?.h ?? 4,
          minW: constraints?.minW,
          minH: constraints?.minH,
        };
      }),
    [pins],
  );

  // Persist only real moves: compare against the last layout we saw.
  const lastSerialized = useRef("");
  const handleLayoutChange = useCallback(
    (next: Layout) => {
      const serialized = JSON.stringify(
        next.map((item: LayoutItem) => [item.i, item.x, item.y, item.w, item.h]),
      );
      if (lastSerialized.current === "") {
        lastSerialized.current = serialized;
        return;
      }
      if (serialized === lastSerialized.current) return;
      lastSerialized.current = serialized;

      layoutMutation.mutate({
        body: {
          layouts: next.map((item: LayoutItem) => ({
            pin_id: item.i,
            x: item.x,
            y: item.y,
            w: item.w,
            h: item.h,
          })),
        },
      });
    },
    [layoutMutation],
  );

  if (!pinsQuery.isLoading && pins.length === 0) {
    return (
      <div className="flex h-full items-center justify-center">
        <div className="text-center text-base-content/50 max-w-sm space-y-2">
          <Pin className="mx-auto" size={28} strokeWidth={1.25} />
          <p>
            {t(
              "lumilio.board.empty",
              "Nothing pinned yet. Ask Lumilio for photos in the chat, then pin the result to keep it here.",
            )}
          </p>
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="h-full overflow-y-auto p-4">
      {mounted && (
        <GridLayout
          width={width}
          layout={layout}
          gridConfig={{ cols: BOARD_COLS, rowHeight: ROW_HEIGHT }}
          dragConfig={{ handle: ".lumilio-widget-drag" }}
          onLayoutChange={handleLayoutChange}
        >
          {pins.map((pin) => (
            <div
              key={pin.pin_id}
              className="rounded-xl border border-base-300 bg-base-100 shadow-sm overflow-hidden flex flex-col"
            >
              <BoardCellHeader
                pin={pin}
                onDelete={() =>
                  deleteMutation.mutate({
                    params: { path: { id: pin.pin_id! } },
                  })
                }
              />
              <div className="flex-1 min-h-0">
                <BoardCellBody pin={pin} />
              </div>
            </div>
          ))}
        </GridLayout>
      )}
    </div>
  );
}

function BoardCellHeader({
  pin,
  onDelete,
}: {
  pin: AgentPinDTO;
  onDelete: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="lumilio-widget-drag flex items-center justify-between gap-2 px-3 py-2 border-b border-base-300 cursor-move select-none shrink-0">
      <div className="text-sm font-medium truncate text-base-content/80">
        {pin.title || t("lumilio.board.untitled", "Untitled")}
        <span className="ml-2 text-xs font-normal text-base-content/40">
          {pin.count}
          {pin.mode === "live" && " · live"}
        </span>
      </div>
      <button
        className="btn btn-ghost btn-xs btn-circle text-base-content/40 hover:text-error"
        onMouseDown={(e) => e.stopPropagation()}
        onClick={onDelete}
        title={t("lumilio.board.remove", "Remove from board")}
      >
        <Trash2 size={14} strokeWidth={1.5} />
      </button>
    </div>
  );
}

function BoardCellBody({ pin }: { pin: AgentPinDTO }) {
  const { t } = useI18n();
  const definition = getWidget(pin.widget ?? "");
  if (!definition) {
    return (
      <div className="h-full flex items-center justify-center text-xs text-base-content/50 p-3">
        {t("lumilio.board.unknownWidget", "Unknown widget type: {{type}}", {
          type: pin.widget,
        })}
      </div>
    );
  }
  const Component = definition.Component;
  return (
    <Component
      source={{ kind: "pin", pinId: pin.pin_id! }}
      variant="board"
      count={pin.count ?? 0}
      title={pin.title ?? undefined}
    />
  );
}
