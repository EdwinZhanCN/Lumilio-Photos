import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent,
} from "react";
import { createPortal } from "react-dom";
import { Plus, Tags as TagsIcon } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import TagPickerMenu, {
  type TagPickerItem,
} from "@/features/assets/components/shared/TagPickerMenu";
import { useAssetTags, isManualTag, type AssetTag } from "@/features/assets/hooks/useAssetTags";

type TagSuggestion = components["schemas"]["dto.TagDTO"];

interface TagListProps {
  assetId?: string;
}

const POPOVER_WIDTH = 256;
const POPOVER_MAX_HEIGHT = 288;
const VIEWPORT_MARGIN = 8;

/**
 * Tags shown in the BasicInfo panel as an inset section. Manual tags are
 * managed through a Linear-style popover (search + checkable list + "create new
 * tag"); AI-generated tags are read-only and only appear in the display row.
 */
export default function TagList({ assetId }: TagListProps) {
  const { t } = useI18n();
  const { tags, isLoading, addTag, removeTag } = useAssetTags(assetId);

  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [style, setStyle] = useState<CSSProperties | null>(null);

  const triggerRef = useRef<HTMLButtonElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  const manualTags = tags.filter(isManualTag);
  const appliedIds = new Set(tags.map((tag: AssetTag) => tag.tag_id));
  const appliedNames = new Set(tags.map((tag: AssetTag) => (tag.tag_name ?? "").toLowerCase()));

  // Autocomplete suggestions from the whole tag library; only while open.
  const suggestionsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/tags",
    { params: { query: { q: query, limit: 20 } } },
    { enabled: open, staleTime: 30_000 },
  );
  const rawSuggestions: TagSuggestion[] = suggestionsQuery.data?.tags ?? [];
  const suggestions = rawSuggestions.filter(
    (tag) => tag.tag_id != null && !appliedIds.has(tag.tag_id),
  );

  const trimmed = query.trim();
  const exactExists =
    trimmed.length > 0 && appliedNames.has(trimmed.toLowerCase())
      ? true
      : suggestions.some((tag) => (tag.tag_name ?? "").toLowerCase() === trimmed.toLowerCase());
  const showCreate = trimmed.length > 0 && !exactExists;

  // Position the portal popover relative to the trigger, flipping above when
  // there isn't room below (the panel content area clips absolute children).
  const reposition = () => {
    const el = triggerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const left = Math.max(
      VIEWPORT_MARGIN,
      Math.min(r.right - POPOVER_WIDTH, window.innerWidth - POPOVER_WIDTH - VIEWPORT_MARGIN),
    );
    const spaceBelow = window.innerHeight - r.bottom;
    const next: CSSProperties = {
      position: "fixed",
      left,
      width: POPOVER_WIDTH,
      // Must sit above the fullscreen overlay (z-9999) it portals out of.
      zIndex: 10000,
    };
    if (spaceBelow >= POPOVER_MAX_HEIGHT || spaceBelow >= r.top) {
      next.top = r.bottom + 4;
    } else {
      next.bottom = window.innerHeight - r.top + 4;
    }
    setStyle(next);
  };

  useLayoutEffect(() => {
    if (!open) return;
    reposition();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const onScrollOrResize = () => reposition();
    const onPointerDown = (e: MouseEvent) => {
      const target = e.target as Node;
      if (popoverRef.current?.contains(target) || triggerRef.current?.contains(target)) {
        return;
      }
      setOpen(false);
    };
    window.addEventListener("scroll", onScrollOrResize, true);
    window.addEventListener("resize", onScrollOrResize);
    document.addEventListener("mousedown", onPointerDown);
    return () => {
      window.removeEventListener("scroll", onScrollOrResize, true);
      window.removeEventListener("resize", onScrollOrResize);
      document.removeEventListener("mousedown", onPointerDown);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const toggleOpen = () => {
    setQuery("");
    setOpen((prev) => !prev);
  };

  const handleCreate = async () => {
    if (!trimmed) return;
    await addTag(trimmed);
    setQuery("");
  };

  const onInputKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Escape") {
      e.preventDefault();
      setOpen(false);
      return;
    }
    if (e.key === "Enter") {
      e.preventDefault();
      if (showCreate) {
        void handleCreate();
      } else if (suggestions[0]?.tag_name) {
        void addTag(suggestions[0].tag_name);
      }
    }
  };

  const checkedItems: TagPickerItem[] = manualTags.map((tag) => ({
    id: tag.tag_id!,
    name: tag.tag_name ?? "",
  }));
  const suggestionItems: TagPickerItem[] = suggestions.map((tag) => ({
    id: tag.tag_id!,
    name: tag.tag_name ?? "",
  }));

  return (
    <div className="rounded bg-base-200 p-3">
      <div className="flex items-center gap-1.5 text-[10px] uppercase tracking-wider font-bold text-base-content/50 mb-2">
        <TagsIcon className="w-3.5 h-3.5 text-base-content" />
        {t("assets.tags.title")}
      </div>

      <div className="flex items-center gap-1.5">
        <div className="flex items-center gap-1.5 overflow-x-auto custom-scrollbar flex-1 min-w-0 py-0.5">
          {isLoading && tags.length === 0 ? (
            <span className="loading loading-spinner loading-xs shrink-0" />
          ) : tags.length === 0 ? (
            <span className="text-xs text-base-content/40 font-sans">{t("assets.tags.empty")}</span>
          ) : (
            tags.map((tag: AssetTag) => (
              <span
                key={tag.tag_id}
                className="badge badge-sm badge-neutral shrink-0 font-sans whitespace-nowrap"
              >
                {tag.tag_name}
              </span>
            ))
          )}
        </div>

        <button
          ref={triggerRef}
          type="button"
          className="btn btn-xs btn-circle btn-ghost shrink-0"
          aria-label={t("assets.tags.add")}
          disabled={!assetId}
          onClick={toggleOpen}
        >
          <Plus className="w-4 h-4" />
        </button>
      </div>

      {open &&
        style &&
        createPortal(
          <div ref={popoverRef} style={style}>
            <TagPickerMenu
              className="shadow-lg"
              style={{ maxHeight: POPOVER_MAX_HEIGHT }}
              autoFocus
              query={query}
              onQueryChange={setQuery}
              onKeyDown={onInputKeyDown}
              placeholder={t("assets.tags.searchPlaceholder")}
              loading={suggestionsQuery.isFetching}
              loadingText={t("assets.tags.loading")}
              noResultsText={t("assets.tags.noResults")}
              checked={checkedItems}
              suggestions={suggestionItems}
              onToggleChecked={(item) => void removeTag(Number(item.id))}
              onSelectSuggestion={(item) => void addTag(item.name)}
              showCreate={showCreate}
              createLabel={t("assets.tags.create")}
              createName={trimmed}
              onCreate={() => void handleCreate()}
            />
          </div>,
          document.body,
        )}
    </div>
  );
}
