import { useNavigate } from "react-router-dom";
import { useQueryClient } from "@tanstack/react-query";
import { useI18n } from "@/lib/i18n.tsx";
import { getWidget, sizeKeyFor } from "../registry";
import type { AgentPinDTO, WidgetSizeKey, WidgetSource } from "../types";
import { useWidgetData } from "../useWidgetData";
import { EmptyState, ErrorState, LoadingState } from "../views/states";
import { TileBody } from "./TileBody";
import { TileHeader } from "./TileHeader";
import { ViewSwitcher } from "./ViewSwitcher";

interface BoardTileProps {
  pin: AgentPinDTO;
  onRename: (title: string) => void;
  onViewChange: (view: string) => void;
  onSize: (size: WidgetSizeKey) => void;
  onRemove: () => void;
}

/** A durable board cell: pin → widget data → the selected view, wrapped in the
 * shared tile chrome. Cover (when ready) uses a glass header floating over the
 * photo; every other view/state uses a solid header. */
export function BoardTile({ pin, onRename, onViewChange, onSize, onRemove }: BoardTileProps) {
  const { t, i18n } = useI18n();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const view = pin.widget ?? "";
  const definition = getWidget(view);
  const size = sizeKeyFor(pin.layout?.w ?? 4, pin.layout?.h ?? 4);
  const pinId = pin.pin_id ?? "";
  const source: WidgetSource = { kind: "pin", pinId };
  const data = useWidgetData(source, {
    count: pin.count ?? 0,
    title: pin.title ?? undefined,
    mode: pin.mode ?? "frozen",
  });

  const cardCls =
    "card group relative flex h-full flex-col overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-sm transition-all hover:border-base-content/15 hover:shadow-md";

  if (!definition) {
    return (
      <div className={cardCls}>
        <div className="grid h-full place-items-center p-3 text-center text-xs text-base-content/50">
          {t("lumilio.board.unknownWidget", "Unknown widget type: {{type}}", { type: view })}
        </div>
      </div>
    );
  }

  const View = definition.View;
  // Refetch the pin's facet/asset metadata (loose key match across its options).
  const retry = () =>
    void queryClient.invalidateQueries({ queryKey: ["get", "/api/v1/agent/pins/{id}"] });
  const activate = () => {
    if (!pinId) return;
    // Carry the board origin so the library can offer a "back to board" crumb.
    // It is router state (not a path segment) because the board isn't a URL
    // parent of /assets — it's where the user came from.
    void navigate(`/assets?pin=${pinId}`, {
      state: {
        from: "/lumilio",
        fromLabel: t("lumilio.nav.board", "Board"),
        label: pin.title ?? undefined,
      },
    });
  };

  let body;
  if (data.state === "loading") body = <LoadingState view={view} size={size} />;
  else if (data.state === "error") body = <ErrorState size={size} onRetry={retry} />;
  else if (data.state === "empty") body = <EmptyState size={size} />;
  else
    body = (
      <View
        data={data}
        size={size}
        ctx={view === "cover_card" ? "glass" : "board"}
        source={source}
      />
    );

  const isGlass = view === "cover_card" && data.state === "ready";

  const hoverSwitcher =
    size === "s" && data.state === "ready" ? (
      <div
        className="absolute bottom-1.5 left-1/2 z-20 -translate-x-1/2 opacity-0 transition-opacity group-hover:opacity-100"
        onMouseDown={(e) => e.stopPropagation()}
      >
        <ViewSwitcher
          current={view}
          onChange={onViewChange}
          variant={isGlass ? "glass" : "default"}
          size="xs"
        />
      </div>
    ) : null;

  const header = (
    <TileHeader
      data={data}
      size={size}
      variant={isGlass ? "glass" : "solid"}
      view={view}
      onViewChange={onViewChange}
      onRenameCommit={onRename}
      onSize={onSize}
      onRemove={onRemove}
      locale={i18n.language}
    />
  );

  if (isGlass) {
    return (
      <div className={cardCls}>
        <TileBody clickable onActivate={activate}>
          {body}
          {header}
          {hoverSwitcher}
        </TileBody>
      </div>
    );
  }

  return (
    <div className={cardCls}>
      {header}
      <TileBody clickable onActivate={activate}>
        {body}
        {hoverSwitcher}
      </TileBody>
    </div>
  );
}
