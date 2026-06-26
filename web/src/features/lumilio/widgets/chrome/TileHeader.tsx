import { useEffect, useRef, useState } from "react";
import { useI18n } from "@/lib/i18n.tsx";
import { fmt } from "../format";
import type { WidgetData, WidgetSizeKey } from "../types";
import { LiveBadge } from "./LiveBadge";
import { MoreMenu } from "./MoreMenu";
import { ViewSwitcher } from "./ViewSwitcher";

interface TileHeaderProps {
  data: WidgetData;
  size: WidgetSizeKey;
  variant: "solid" | "glass";
  view: string;
  onViewChange: (view: string) => void;
  onRenameCommit: (title: string) => void;
  onSize: (size: WidgetSizeKey) => void;
  onRemove: () => void;
  locale?: string;
}

/** Title text + inline rename. Display is double-click-to-edit; editing commits
 * on Enter / blur and cancels on Escape. Empty input keeps the current title. */
function useRename(current: string | undefined, onCommit: (title: string) => void) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(current ?? "");
  const inputRef = useRef<HTMLInputElement>(null);

  const begin = () => {
    setDraft(current ?? "");
    setEditing(true);
  };
  const commit = () => {
    setEditing(false);
    const next = draft.trim();
    if (next && next !== (current ?? "")) onCommit(next);
  };
  const cancel = () => setEditing(false);

  useEffect(() => {
    if (editing) inputRef.current?.select();
  }, [editing]);

  return { editing, draft, setDraft, begin, commit, cancel, inputRef };
}

function RenameInput({
  rename,
  onMouseDown,
}: {
  rename: ReturnType<typeof useRename>;
  onMouseDown?: (e: React.MouseEvent) => void;
}) {
  const { t } = useI18n();
  return (
    <input
      ref={rename.inputRef}
      value={rename.draft}
      autoFocus
      onChange={(e) => rename.setDraft(e.target.value)}
      onBlur={rename.commit}
      onClick={(e) => e.stopPropagation()}
      onMouseDown={onMouseDown ?? ((e) => e.stopPropagation())}
      onKeyDown={(e) => {
        if (e.key === "Enter") rename.commit();
        else if (e.key === "Escape") rename.cancel();
      }}
      className="input input-xs w-full max-w-[160px] bg-base-100 font-semibold text-base-content focus:outline-primary"
      aria-label={t("lumilio.widgets.menu.rename", "Rename")}
    />
  );
}

/** The tile chrome bar. Solid (Stat/Timeline/Mosaic) is a bordered header;
 * glass (Cover) floats over the photo and shows no title text — the caption in
 * the body carries the title, and rename is triggered from the menu. All
 * controls stopPropagation so they never start a grid drag or trip the body
 * deep-link. */
export function TileHeader({
  data,
  size,
  variant,
  view,
  onViewChange,
  onRenameCommit,
  onSize,
  onRemove,
  locale,
}: TileHeaderProps) {
  const { t } = useI18n();
  const rename = useRename(data.title, onRenameCommit);
  const showSwitcher = size !== "s"; // S uses the body hover overlay instead
  const switcherSize = size === "l" ? "sm" : "xs";
  const stop = (e: React.MouseEvent) => e.stopPropagation();

  const menu = (
    <MoreMenu
      currentSize={size}
      onRename={rename.begin}
      onSize={onSize}
      onRemove={onRemove}
      glass={variant === "glass"}
    />
  );

  if (variant === "glass") {
    return (
      <>
        <div
          className="lumilio-widget-drag absolute inset-x-0 top-0 z-20 flex cursor-move items-center gap-1 p-1.5"
          onClick={stop}
        >
          <LiveBadge data={data} size={size} locale={locale} />
          <div className="flex-1" />
          {showSwitcher && (
            <ViewSwitcher current={view} onChange={onViewChange} variant="glass" size={switcherSize} />
          )}
          {menu}
        </div>
        {rename.editing && (
          <div className="absolute inset-x-2 top-9 z-30" onMouseDown={stop}>
            <RenameInput rename={rename} />
          </div>
        )}
      </>
    );
  }

  return (
    <div
      className={`lumilio-widget-drag flex shrink-0 cursor-move items-center gap-1.5 border-b border-base-200 bg-base-100 ${
        size === "s" ? "h-8 px-1.5" : "h-10 px-2.5"
      }`}
    >
      <div className="flex min-w-0 flex-1 items-center gap-1.5">
        {rename.editing ? (
          <RenameInput rename={rename} />
        ) : (
          <span
            className="cursor-text truncate font-bold text-base-content"
            title={t("lumilio.widgets.menu.renameHint", "Double-click to rename")}
            onDoubleClick={(e) => {
              e.stopPropagation();
              rename.begin();
            }}
          >
            {data.title || t("lumilio.board.untitled", "Untitled")}
          </span>
        )}
        {size !== "s" && !rename.editing && (
          <span className="shrink-0 text-xs tabular-nums text-base-content/45">
            {fmt(data.count, locale)}
          </span>
        )}
        <LiveBadge data={data} size={size} locale={locale} />
      </div>
      <div className="flex shrink-0 items-center gap-1">
        {showSwitcher && (
          <ViewSwitcher current={view} onChange={onViewChange} size={switcherSize} />
        )}
        {menu}
      </div>
    </div>
  );
}
