import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  GridLayout,
  noCompactor,
  useContainerWidth,
  verticalCompactor,
  type Layout,
  type LayoutItem,
} from "react-grid-layout";
import "react-grid-layout/css/styles.css";
import { AlertTriangle, LayoutDashboard, RotateCcw, Sparkles, Undo2 } from "lucide-react";
import { useQueryClient } from "@tanstack/react-query";
import { $api } from "@/lib/http-commons/queryClient";
import { useI18n } from "@/lib/i18n.tsx";
import { localizeAPIProblem } from "@/lib/http-commons/problem";
import { BoardTile } from "../../modules/widgets/chrome/BoardTile";
import { DIMS, getWidget } from "../../modules/widgets/registry";
import type { AgentPinDTO, WidgetSizeKey } from "../../modules/widgets/types";

const BOARD_COLS = 12;
const ROW_HEIGHT = 72;
const MOBILE_BREAKPOINT = 760;
const LAYOUT_WRITE_DELAY_MS = 400;
const DELETE_UNDO_MS = 5000;
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

type LayoutMutationContext = { previousPins?: AgentPinDTO[] };

type PendingDelete = {
  pin: AgentPinDTO;
  timer: ReturnType<typeof setTimeout>;
};

function serializeLayout(layout: Layout) {
  return JSON.stringify(layout.map((item: LayoutItem) => [item.i, item.x, item.y, item.w, item.h]));
}

/** Durable Agent workspace. Desktop layout is canonical; narrow screens render
 * a read-safe single column and never write responsive coordinates back into
 * the desktop grid. */
export function AgentBoard() {
  const { t } = useI18n();
  const queryClient = useQueryClient();
  const { width, containerRef, mounted } = useContainerWidth();
  const [operationError, setOperationError] = useState<string | null>(null);
  const [pendingDelete, setPendingDelete] = useState<PendingDelete | null>(null);
  const layoutWriteTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const pinsQuery = $api.useQuery("get", "/api/v1/agent/pins", {});
  const pins = useMemo(() => pinsQuery.data ?? [], [pinsQuery.data]);
  const isNarrow = mounted && width > 0 && width < MOBILE_BREAKPOINT;
  const mutationMessage = (error: unknown) =>
    localizeAPIProblem(
      error,
      t,
      t(
        "lumilio.board.changeFailed",
        "The board change could not be saved. Your previous layout was restored.",
      ),
    );

  const layoutMutation = $api.useMutation("patch", "/api/v1/agent/pins/layout", {
    onMutate: async (variables) => {
      setOperationError(null);
      const body = (variables as LayoutPatchVariables | undefined)?.body;
      if (!body) return {};

      await queryClient.cancelQueries({ queryKey: PINS_QUERY_KEY });
      const previousPins = queryClient.getQueryData<AgentPinDTO[]>(PINS_QUERY_KEY);
      const nextLayouts = new Map(body.layouts.map((item) => [item.pin_id, item]));
      queryClient.setQueryData<AgentPinDTO[] | undefined>(PINS_QUERY_KEY, (current) =>
        current?.map((pin) => {
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
        }),
      );
      return { previousPins };
    },
    onError: (error, _variables, context) => {
      const typedContext = context as LayoutMutationContext | undefined;
      if (typedContext?.previousPins)
        queryClient.setQueryData(PINS_QUERY_KEY, typedContext.previousPins);
      setOperationError(mutationMessage(error));
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: PINS_QUERY_KEY }),
  });

  const deleteMutation = $api.useMutation("delete", "/api/v1/agent/pins/{id}", {
    onError: (error) => setOperationError(mutationMessage(error)),
    onSettled: () => {
      setPendingDelete(null);
      void queryClient.invalidateQueries({ queryKey: PINS_QUERY_KEY });
    },
  });

  const optimisticPinMutation = (field: "title" | "widget") => ({
    onMutate: async (variables: unknown) => {
      setOperationError(null);
      const vars = variables as
        | { params: { path: { id: string } }; body: Record<string, string> }
        | undefined;
      if (!vars) return {};
      await queryClient.cancelQueries({ queryKey: PINS_QUERY_KEY });
      const previousPins = queryClient.getQueryData<AgentPinDTO[]>(PINS_QUERY_KEY);
      queryClient.setQueryData<AgentPinDTO[] | undefined>(PINS_QUERY_KEY, (current) =>
        current?.map((pin) =>
          pin.pin_id === vars.params.path.id ? { ...pin, [field]: vars.body[field] } : pin,
        ),
      );
      return { previousPins };
    },
    onError: (error: unknown, _variables: unknown, context: unknown) => {
      const typedContext = context as LayoutMutationContext | undefined;
      if (typedContext?.previousPins)
        queryClient.setQueryData(PINS_QUERY_KEY, typedContext.previousPins);
      setOperationError(mutationMessage(error));
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: PINS_QUERY_KEY }),
  });

  const renameMutation = $api.useMutation(
    "patch",
    "/api/v1/agent/pins/{id}",
    optimisticPinMutation("title"),
  );
  const updateViewMutation = $api.useMutation(
    "patch",
    "/api/v1/agent/pins/{id}",
    optimisticPinMutation("widget"),
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
  const serializedLayout = useMemo(() => serializeLayout(layout), [layout]);
  const lastSerialized = useRef("");
  useEffect(() => {
    lastSerialized.current = serializedLayout;
  }, [serializedLayout]);

  const pendingDeleteRef = useRef<PendingDelete | null>(null);

  useEffect(() => {
    pendingDeleteRef.current = pendingDelete;
  }, [pendingDelete]);

  useEffect(
    () => () => {
      if (layoutWriteTimer.current) clearTimeout(layoutWriteTimer.current);
      if (pendingDeleteRef.current) clearTimeout(pendingDeleteRef.current.timer);
    },
    [],
  );

  const persistLayout = useCallback(
    (next: Layout) => {
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

  const handleLayoutChange = useCallback(
    (next: Layout) => {
      if (isNarrow) return;
      const serialized = serializeLayout(next);
      if (serialized === lastSerialized.current) return;
      lastSerialized.current = serialized;
      if (layoutWriteTimer.current) clearTimeout(layoutWriteTimer.current);
      const snapshot = next.map((item) => ({ ...item }));
      layoutWriteTimer.current = setTimeout(() => persistLayout(snapshot), LAYOUT_WRITE_DELAY_MS);
    },
    [isNarrow, persistLayout],
  );

  const patchLayout = useCallback(
    (pin: AgentPinDTO, w: number, h: number) => {
      if (isNarrow) {
        setOperationError(
          t(
            "lumilio.board.desktopSizeOnly",
            "Open the board on a wider screen to change its desktop tile size.",
          ),
        );
        return;
      }
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
    [isNarrow, layoutMutation, t],
  );

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

  const handleTidy = useCallback(() => {
    if (layout.length === 0 || isNarrow) return;
    persistLayout(verticalCompactor.compact(layout, BOARD_COLS));
  }, [isNarrow, layout, persistLayout]);

  const scheduleDelete = useCallback(
    (pin: AgentPinDTO) => {
      if (!pin.pin_id) return;
      if (pendingDelete) clearTimeout(pendingDelete.timer);
      setOperationError(null);
      const timer = setTimeout(() => {
        deleteMutation.mutate({ params: { path: { id: pin.pin_id! } } });
      }, DELETE_UNDO_MS);
      setPendingDelete({ pin, timer });
    },
    [deleteMutation, pendingDelete],
  );

  const undoDelete = useCallback(() => {
    if (!pendingDelete) return;
    clearTimeout(pendingDelete.timer);
    setPendingDelete(null);
  }, [pendingDelete]);

  const renderTile = (pin: AgentPinDTO) => (
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
      onRemove={() => scheduleDelete(pin)}
    />
  );

  if (pinsQuery.isLoading) {
    return (
      <div className="grid h-full place-items-center" role="status">
        <span className="loading loading-spinner loading-md text-primary" />
      </div>
    );
  }

  if (pinsQuery.isError) {
    return (
      <div className="grid h-full place-items-center p-6">
        <div className="max-w-md rounded-box border border-error/30 bg-error/5 p-5 text-center">
          <AlertTriangle className="mx-auto mb-2 text-error" />
          <p className="text-sm text-base-content/75">
            {t("lumilio.board.loadFailed", "The Agent board could not be loaded.")}
          </p>
          <button
            className="btn btn-sm mt-3"
            type="button"
            onClick={() => void pinsQuery.refetch()}
          >
            <RotateCcw size={14} />
            {t("common.retry", "Retry")}
          </button>
        </div>
      </div>
    );
  }

  if (pins.length === 0) {
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
              "Ask Lumilio Agent for photos, then pin any result here to keep it as a live widget.",
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
      {operationError && (
        <div className="alert alert-error mb-3 text-sm" role="alert">
          <AlertTriangle size={16} />
          <span>{operationError}</span>
          <button
            type="button"
            className="btn btn-ghost btn-xs"
            onClick={() => setOperationError(null)}
          >
            {t("common.dismiss", "Dismiss")}
          </button>
        </div>
      )}
      {pendingDelete && (
        <div className="alert alert-warning mb-3 text-sm" role="status">
          <span>
            {t("lumilio.board.deletePending", {
              defaultValue: "“{{title}}” will be removed from the board in 5 seconds.",
              title: pendingDelete.pin.title ?? t("lumilio.board.untitled", "Untitled pin"),
            })}
          </span>
          <button type="button" className="btn btn-sm" onClick={undoDelete}>
            <Undo2 size={14} />
            {t("common.undo", "Undo")}
          </button>
        </div>
      )}

      <div className="flex items-center justify-end pb-2">
        {!isNarrow && (
          <button
            type="button"
            className="btn btn-ghost btn-xs gap-1.5 text-base-content/60 hover:text-base-content"
            onClick={handleTidy}
            title={t("lumilio.board.tidyHint", "Compact the board upward")}
          >
            <Sparkles size={14} strokeWidth={1.5} />
            {t("lumilio.board.tidy", "Tidy")}
          </button>
        )}
      </div>

      {isNarrow ? (
        <div className="grid grid-cols-1 gap-3">
          {pins.map((pin) => (
            <div key={pin.pin_id} className="h-[22rem] min-h-72">
              {renderTile(pin)}
            </div>
          ))}
        </div>
      ) : (
        mounted && (
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
              <div key={pin.pin_id}>{renderTile(pin)}</div>
            ))}
          </GridLayout>
        )
      )}
      <p className="pt-2 text-center text-xs text-base-content/30">
        {isNarrow
          ? t("lumilio.board.mobileHint", "Desktop layout is preserved on this narrow screen.")
          : t(
              "lumilio.board.interactionHint",
              "Drag to move · use the View buttons to switch the look · the ⋯ menu sets size",
            )}
      </p>
    </div>
  );
}
