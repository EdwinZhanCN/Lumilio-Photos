import { useCallback, useEffect, useMemo, useRef } from "react";
import {
  GridLayout,
  noCompactor,
  useContainerWidth,
  verticalCompactor,
  type Layout,
  type LayoutItem,
} from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import { LayoutDashboard, Sparkles } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { BoardTile } from "../../widgets/chrome/BoardTile";
import { DIMS, getWidget } from "../../widgets/registry";
import type { AgentPinDTO, WidgetSizeKey } from "../../widgets/types";

const BOARD_COLS = 12;
const ROW_HEIGHT = 72;
const PINS_QUERY_KEY = ["get", "/api/v1/agent/pins"] as const;

type LayoutPatchVariables = {
  body: {
    layouts: {
      pin_id: string;
      x?: number;
      y?: number;
      w?: number;
      h?: number;
    }[];
  };
};

type LayoutMutationContext = {
  previousPins?: AgentPinDTO[];
};

function serializeLayout(layout: Layout) {
  return JSON.stringify(layout.map((item: LayoutItem) => [item.i, item.x, item.y, item.w, item.h]));
}

/** The durable widget board: every cell is a pinned ref rendered through the
 * widget registry, draggable via react-grid-layout. Cells no longer free-resize
 * — size is a discrete S/M/L tier picked from the cell menu, and each view
 * styles itself per tier. Layout changes persist to the pins API. */
export function AgentBoard() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { width, containerRef, mounted } = useContainerWidth();

  const pinsQuery = $api.useQuery("get", "/api/v1/agent/pins", {});
  const pins = useMemo(() => pinsQuery.data ?? [], [pinsQuery.data]);

  const layoutMutation = $api.useMutation("patch", "/api/v1/agent/pins/layout", {
    onMutate: async (variables) => {
      const body = (variables as LayoutPatchVariables | undefined)?.body;
      if (!body) return {};

      await queryClient.cancelQueries({ queryKey: PINS_QUERY_KEY });
      const previousPins = queryClient.getQueryData<AgentPinDTO[]>(PINS_QUERY_KEY);
      const nextLayouts = new Map(body.layouts.map((item) => [item.pin_id, item]));

      queryClient.setQueryData<AgentPinDTO[] | undefined>(PINS_QUERY_KEY, (current) => {
        if (!current) return current;
        return current.map((pin) => {
          const next = nextLayouts.get(pin.pin_id ?? "");
          if (!next) return pin;
          return {
            ...pin,
            layout: {
              x: next.x ?? pin.layout?.x ?? 0,
              y: next.y ?? pin.layout?.y ?? 0,
              w: next.w ?? pin.layout?.w ?? 4,
              h: next.h ?? pin.layout?.h ?? 4,
            },
          };
        });
      });

      return { previousPins };
    },
    onError: (_error, _variables, context) => {
      const typedContext = context as LayoutMutationContext | undefined;
      if (typedContext?.previousPins) {
        queryClient.setQueryData(PINS_QUERY_KEY, typedContext.previousPins);
      }
    },
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: PINS_QUERY_KEY,
      }),
  });
  const deleteMutation = $api.useMutation("delete", "/api/v1/agent/pins/{id}", {
    onSuccess: () =>
      void queryClient.invalidateQueries({
        queryKey: PINS_QUERY_KEY,
      }),
  });
  const renameMutation = $api.useMutation("patch", "/api/v1/agent/pins/{id}", {
    onMutate: async (variables) => {
      const vars = variables as
        | { params: { path: { id: string } }; body: { title: string } }
        | undefined;
      if (!vars) return {};
      await queryClient.cancelQueries({ queryKey: PINS_QUERY_KEY });
      const previousPins = queryClient.getQueryData<AgentPinDTO[]>(PINS_QUERY_KEY);
      queryClient.setQueryData<AgentPinDTO[] | undefined>(PINS_QUERY_KEY, (current) =>
        current?.map((pin) =>
          pin.pin_id === vars.params.path.id ? { ...pin, title: vars.body.title } : pin,
        ),
      );
      return { previousPins };
    },
    onError: (_error, _variables, context) => {
      const typedContext = context as LayoutMutationContext | undefined;
      if (typedContext?.previousPins) {
        queryClient.setQueryData(PINS_QUERY_KEY, typedContext.previousPins);
      }
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: PINS_QUERY_KEY }),
  });
  const updateViewMutation = $api.useMutation("patch", "/api/v1/agent/pins/{id}", {
    onMutate: async (variables) => {
      const vars = variables as
        | { params: { path: { id: string } }; body: { widget: string } }
        | undefined;
      if (!vars) return {};
      await queryClient.cancelQueries({ queryKey: PINS_QUERY_KEY });
      const previousPins = queryClient.getQueryData<AgentPinDTO[]>(PINS_QUERY_KEY);
      queryClient.setQueryData<AgentPinDTO[] | undefined>(PINS_QUERY_KEY, (current) =>
        current?.map((pin) =>
          pin.pin_id === vars.params.path.id ? { ...pin, widget: vars.body.widget } : pin,
        ),
      );
      return { previousPins };
    },
    onError: (_error, _variables, context) => {
      const typedContext = context as LayoutMutationContext | undefined;
      if (typedContext?.previousPins) {
        queryClient.setQueryData(PINS_QUERY_KEY, typedContext.previousPins);
      }
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: PINS_QUERY_KEY }),
  });

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
  const serializedLayout = useMemo(() => serializeLayout(layout), [layout]);

  // Persist only real user changes: compare against the last server/cache layout.
  const lastSerialized = useRef("");
  useEffect(() => {
    lastSerialized.current = serializedLayout;
  }, [serializedLayout]);

  const handleLayoutChange = useCallback(
    (next: Layout) => {
      const serialized = serializeLayout(next);
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

  // Persist one pin's grid cell directly (size presets, view-switch snap).
  const patchLayout = useCallback(
    (pin: AgentPinDTO, w: number, h: number) => {
      if (w === (pin.layout?.w ?? 4) && h === (pin.layout?.h ?? 4)) return;
      layoutMutation.mutate({
        body: {
          layouts: [
            {
              pin_id: pin.pin_id!,
              x: pin.layout?.x ?? 0,
              y: pin.layout?.y ?? 0,
              w,
              h,
            },
          ],
        },
      });
    },
    [layoutMutation],
  );

  // Switch which view a cell renders through and persist it. All views share one
  // footprint per tier (DIMS), so the cell never needs re-fitting on switch.
  const handleViewChange = useCallback(
    (pin: AgentPinDTO, nextView: string) => {
      if (nextView === pin.widget) return;
      updateViewMutation.mutate({
        params: { path: { id: pin.pin_id! } },
        body: { widget: nextView },
      });
    },
    [updateViewMutation],
  );

  const handleSizePreset = useCallback(
    (pin: AgentPinDTO, preset: WidgetSizeKey) => {
      const size = DIMS[preset];
      patchLayout(pin, size.w, size.h);
    },
    [patchLayout],
  );

  // One-shot tidy: compact the free layout upward and persist the result. The
  // board itself stays free (noCompactor) — this is a deliberate user action.
  const handleTidy = useCallback(() => {
    if (layout.length === 0) return;
    const tidied = verticalCompactor.compact(layout, BOARD_COLS);
    layoutMutation.mutate({
      body: {
        layouts: tidied.map((item: LayoutItem) => ({
          pin_id: item.i,
          x: item.x,
          y: item.y,
          w: item.w,
          h: item.h,
        })),
      },
    });
  }, [layout, layoutMutation]);

  if (!pinsQuery.isLoading && pins.length === 0) {
    return (
      <div className="flex h-full flex-col p-4 pb-40">
        <div className="m-1 flex min-h-[420px] flex-1 flex-col items-center justify-center gap-3 rounded-box border-2 border-dashed border-base-300 px-6 text-center">
          <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-base-200 text-base-content/30">
            <LayoutDashboard size={32} strokeWidth={1.5} />
          </div>
          <div className="text-lg font-bold text-base-content/80">
            {t("lumilio.board.emptyTitle", "Your board is empty")}
          </div>
          <div className="max-w-xs text-sm text-base-content/50">
            {t(
              "lumilio.board.empty",
              "Ask Lumilio for photos, then pin any result here to keep it as a live widget.",
            )}
          </div>
          <div className="flex items-center gap-1.5 rounded-full bg-base-200 px-3 py-1.5 text-xs text-base-content/55">
            <Sparkles className="text-primary" size={14} strokeWidth={1.75} />
            {t("lumilio.board.emptyHint", "Try “Show my beach photos from last summer”")}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="h-full overflow-y-auto p-4 pb-40">
      <div className="flex items-center justify-end pb-2">
        <button
          className="btn btn-ghost btn-xs gap-1.5 text-base-content/60 hover:text-base-content"
          onClick={handleTidy}
          title={t("lumilio.board.tidyHint", "Compact the board upward")}
        >
          <Sparkles size={14} strokeWidth={1.5} />
          {t("lumilio.board.tidy", "Tidy")}
        </button>
      </div>
      {mounted && (
        <GridLayout
          width={width}
          layout={layout}
          gridConfig={{ cols: BOARD_COLS, rowHeight: ROW_HEIGHT }}
          dragConfig={{ enabled: true, handle: ".lumilio-widget-drag" }}
          resizeConfig={{ enabled: false }}
          onLayoutChange={handleLayoutChange}
          compactor={noCompactor}
        >
          {pins.map((pin) => (
            <div key={pin.pin_id}>
              <BoardTile
                pin={pin}
                onRename={(title) =>
                  renameMutation.mutate({
                    params: { path: { id: pin.pin_id! } },
                    body: { title },
                  })
                }
                onViewChange={(view) => handleViewChange(pin, view)}
                onSize={(preset) => handleSizePreset(pin, preset)}
                onRemove={() => deleteMutation.mutate({ params: { path: { id: pin.pin_id! } } })}
              />
            </div>
          ))}
        </GridLayout>
      )}
      <p className="pt-2 text-center text-xs text-base-content/30">
        {t(
          "lumilio.board.interactionHint",
          "Drag to move · use the View buttons to switch the look · the ⋯ menu sets size",
        )}
      </p>
    </div>
  );
}
