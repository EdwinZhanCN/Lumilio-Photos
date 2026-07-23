import {
  useCallback,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent,
  type ReactNode,
} from "react";
import {
  Send,
  User,
  FolderOpen,
  Pin,
  Camera,
  Aperture,
  AtSign,
  Sparkles,
  ChevronRight,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import { useAlbumOptions } from "@/lib/albums/useAlbumOptions";
import { useAssetFilterOptions } from "@/features/assets";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import {
  createMentionSources,
  getEntitiesForType,
  type MentionEntity,
  type MentionPayload,
  type MentionType,
} from "../../modules/mentions/mentionSources";
import { useSlashMacros, type SlashMacro } from "../../modules/slash/slashMacros";

const MAX_TEXTAREA_HEIGHT = 140;

/** Per-type icon + theme-token chip styling for the command palette.
 * Colors are daisyUI semantic tokens only (no hardcoded values); the class
 * strings are literal so Tailwind's JIT keeps them. */
const TYPE_STYLE: Record<MentionType, { icon: ReactNode; chip: string }> = {
  person: { icon: <User size={14} />, chip: "bg-primary/15 text-primary" },
  album: { icon: <FolderOpen size={14} />, chip: "bg-secondary/15 text-secondary" },
  pin: { icon: <Pin size={14} />, chip: "bg-accent/15 text-accent" },
  camera: { icon: <Camera size={14} />, chip: "bg-info/15 text-info" },
  lens: { icon: <Aperture size={14} />, chip: "bg-success/15 text-success" },
};

const MACRO_CHIP = "bg-primary/15 text-primary";

/** Two-level mention machine, mirroring the legacy RichInput phases:
 *  IDLE → (type "@") → "type"  → (pick type) → "entity"
 *  IDLE → (type "/") → "command"
 * `start` is the index of the trigger char; `query` is the text after it. */
type Menu =
  | { kind: "idle" }
  | { kind: "type"; start: number; query: string }
  | { kind: "entity"; activeType: MentionType; start: number; query: string }
  | { kind: "command"; start: number; query: string };

type MenuItem =
  | { kind: "type"; type: MentionType; label: string }
  | { kind: "entity"; entity: MentionEntity }
  | { kind: "macro"; macro: SlashMacro };

/** The @ or / token immediately before the caret (no whitespace in it). */
function activeTrigger(
  text: string,
  caret: number,
): { char: "@" | "/"; start: number; query: string } | null {
  for (let i = caret - 1; i >= 0; i--) {
    const ch = text[i];
    if (ch === "@" || ch === "/") {
      if (i === 0 || /\s/.test(text[i - 1])) {
        const query = text.slice(i + 1, caret);
        if (/\s/.test(query)) return null;
        return { char: ch, start: i, query };
      }
      return null;
    }
    if (/\s/.test(ch)) return null;
  }
  return null;
}

type MentionInputProps = {
  isGenerating: boolean;
  disabled?: boolean;
  placeholder?: string;
  /** Sticky agent mode owned by the parent (null = free mode). */
  activeMode: string | null;
  /** Set/clear the sticky mode (e.g. picking a quick action). */
  onSetMode: (mode: string | null) => void;
  onSubmit: (query: string, mentions: MentionPayload[]) => void;
};

export function MentionInput({
  isGenerating,
  disabled = false,
  placeholder,
  activeMode,
  onSetMode,
  onSubmit,
}: MentionInputProps) {
  const { t } = useI18n();
  const SLASH_MACROS = useSlashMacros();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const pendingCaretRef = useRef<number | null>(null);

  const [value, setValue] = useState("");
  const [mentions, setMentions] = useState<MentionPayload[]>([]);
  const [menu, setMenu] = useState<Menu>({ kind: "idle" });
  const [selectedIndex, setSelectedIndex] = useState(0);

  const peopleQuery = $api.useQuery("get", "/api/v1/people", {
    params: { query: { limit: 200, offset: 0 } },
  });
  const albumsQuery = useAlbumOptions();
  // Pins live behind the LLM-agent gate; when the agent is disabled the endpoint
  // returns 404 and would otherwise trip React Query's default retry loop on
  // every mount. Gate the query on the (cached) capabilities flag instead.
  const { capabilities } = useCapabilities();
  const agentEnabled = capabilities?.llm?.agentEnabled ?? false;
  const pinsQuery = $api.useQuery(
    "get",
    "/api/v1/agent/pins",
    {},
    {
      enabled: agentEnabled,
      retry: false,
    },
  );
  const filterOptionsQuery = useAssetFilterOptions();

  const sources = useMemo(
    () =>
      createMentionSources({
        people: peopleQuery.data?.people ?? [],
        albums: albumsQuery.data?.albums ?? [],
        pins: pinsQuery.data ?? [],
        cameras: filterOptionsQuery.data?.camera_models ?? [],
        lenses: filterOptionsQuery.data?.lenses ?? [],
      }),
    [peopleQuery.data, albumsQuery.data, pinsQuery.data, filterOptionsQuery.data],
  );

  const mentionTypes = useMemo<{ type: MentionType; label: string }[]>(
    () => [
      { type: "person", label: t("lumilio.mention.person", "Person") },
      { type: "album", label: t("lumilio.mention.album", "Album") },
      { type: "pin", label: t("lumilio.mention.pin", "Pin") },
      { type: "camera", label: t("lumilio.mention.camera", "Camera") },
      { type: "lens", label: t("lumilio.mention.lens", "Lens") },
    ],
    [t],
  );

  const items = useMemo<MenuItem[]>(() => {
    if (menu.kind === "type") {
      const q = menu.query.toLowerCase();
      return mentionTypes
        .filter((mt) => !q || mt.label.toLowerCase().includes(q))
        .map((mt) => ({ kind: "type" as const, type: mt.type, label: mt.label }));
    }
    if (menu.kind === "entity") {
      return getEntitiesForType(sources, menu.activeType, menu.query).map((entity) => ({
        kind: "entity" as const,
        entity,
      }));
    }
    if (menu.kind === "command") {
      const q = menu.query.toLowerCase();
      return SLASH_MACROS.filter((m) => m.label.toLowerCase().includes(q) || m.id.includes(q)).map(
        (macro) => ({ kind: "macro" as const, macro }),
      );
    }
    return [];
  }, [menu, mentionTypes, sources, SLASH_MACROS]);

  const menuOpen = menu.kind !== "idle";
  const safeIndex = items.length ? Math.min(selectedIndex, items.length - 1) : 0;

  useLayoutEffect(() => {
    if (pendingCaretRef.current !== null && textareaRef.current) {
      const pos = pendingCaretRef.current;
      pendingCaretRef.current = null;
      textareaRef.current.focus();
      textareaRef.current.setSelectionRange(pos, pos);
    }
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
      textareaRef.current.style.height = `${Math.min(
        textareaRef.current.scrollHeight,
        MAX_TEXTAREA_HEIGHT,
      )}px`;
    }
  }, [value]);

  /** Recompute the menu from the caret, preserving an active entity phase. */
  const syncMenu = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    const caret = el.selectionStart ?? el.value.length;
    const trig = activeTrigger(el.value, caret);
    setMenu((prev) => {
      if (!trig) return { kind: "idle" };
      if (trig.char === "/") {
        return { kind: "command", start: trig.start, query: trig.query };
      }
      // "@": keep the entity phase if we're still on the same trigger token.
      if (prev.kind === "entity" && prev.start === trig.start) {
        return { ...prev, query: trig.query };
      }
      return { kind: "type", start: trig.start, query: trig.query };
    });
    setSelectedIndex(0);
  }, []);

  const handleChange = useCallback(() => {
    const el = textareaRef.current;
    if (!el) return;
    setValue(el.value);
    syncMenu();
  }, [syncMenu]);

  /** Insert a trigger char at the caret and open its menu — used by the
   * toolbar buttons so they share the exact typing flow (type-to-filter,
   * caret tracking) instead of a separate code path. */
  const openTrigger = useCallback(
    (char: "@" | "/") => {
      if (disabled) return;
      const el = textareaRef.current;
      const caret = el?.selectionStart ?? value.length;
      const needsSpace = caret > 0 && !/\s/.test(value[caret - 1] ?? "");
      const insert = `${needsSpace ? " " : ""}${char}`;
      const triggerIdx = caret + insert.length - 1;
      setValue(value.slice(0, caret) + insert + value.slice(caret));
      pendingCaretRef.current = triggerIdx + 1;
      setMenu(
        char === "/"
          ? { kind: "command", start: triggerIdx, query: "" }
          : { kind: "type", start: triggerIdx, query: "" },
      );
      setSelectedIndex(0);
    },
    [disabled, value],
  );

  /** Close the menu; when `strip`, also remove the in-progress trigger token
   * (so Escape cancels the "@…" / "/…" the user was composing). */
  const closeMenu = useCallback(
    (strip: boolean) => {
      setMenu((m) => {
        if (strip && m.kind !== "idle") {
          const ch = value[m.start];
          if (ch === "@" || ch === "/") {
            const caret = textareaRef.current?.selectionStart ?? value.length;
            const end = Math.max(caret, m.start + 1);
            setValue(value.slice(0, m.start) + value.slice(end));
            pendingCaretRef.current = m.start;
          }
        }
        return { kind: "idle" };
      });
    },
    [value],
  );

  const submit = useCallback(() => {
    const text = value.replace(/\s+/g, " ").trim();
    if (!text || disabled || isGenerating) return;
    const kept = mentions.filter((m) => value.includes(`@${m.label}`));
    onSubmit(text, kept);
    setValue("");
    setMentions([]);
    setMenu({ kind: "idle" });
  }, [value, mentions, disabled, isGenerating, onSubmit]);

  const choose = useCallback(
    (item: MenuItem) => {
      if (menu.kind === "idle") return;

      // First level: pick a mention type → drop into the entity sub-menu,
      // clearing whatever was typed after "@" so the entity query starts fresh.
      if (item.kind === "type") {
        const start = menu.start;
        const caret = textareaRef.current?.selectionStart ?? value.length;
        const next = value.slice(0, start + 1) + value.slice(caret);
        setValue(next);
        pendingCaretRef.current = start + 1;
        setMenu({ kind: "entity", activeType: item.type, start, query: "" });
        setSelectedIndex(0);
        return;
      }

      // Quick action: set the sticky mode (parent-owned). Strip the "/query"
      // trigger token; never insert a template — picking a mode only constrains
      // the agent, the user types their own request.
      if (item.kind === "macro") {
        onSetMode(item.macro.mode);
        if (menu.kind === "command" && value[menu.start] === "/") {
          const caret = textareaRef.current?.selectionStart ?? value.length;
          setValue(value.slice(0, menu.start) + value.slice(caret));
          pendingCaretRef.current = menu.start;
        }
        setMenu({ kind: "idle" });
        return;
      }

      // Second level (entity): replace the trigger token with the chosen label.
      const start = menu.start;
      const caret = textareaRef.current?.selectionStart ?? value.length;
      const before = value.slice(0, start);
      const after = value.slice(caret);
      const inserted = `@${item.entity.label} `;
      setValue(before + inserted + after);
      pendingCaretRef.current = before.length + inserted.length;
      setMentions((prev) =>
        prev.some((m) => m.id === item.entity.id && m.type === item.entity.type)
          ? prev
          : [
              ...prev,
              {
                type: item.entity.type,
                id: item.entity.id,
                label: item.entity.label,
              },
            ],
      );
      setMenu({ kind: "idle" });
    },
    [menu, value, onSetMode],
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (menuOpen) {
        if (e.key === "Escape") {
          e.preventDefault();
          closeMenu(true);
          return;
        }
        if (items.length > 0) {
          if (e.key === "ArrowDown") {
            e.preventDefault();
            setSelectedIndex((i) => (i + 1) % items.length);
            return;
          }
          if (e.key === "ArrowUp") {
            e.preventDefault();
            setSelectedIndex((i) => (i - 1 + items.length) % items.length);
            return;
          }
          if (e.key === "Enter" || e.key === "Tab") {
            e.preventDefault();
            choose(items[safeIndex]);
            return;
          }
        } else if (e.key === "Enter" || e.key === "Tab") {
          // Menu open but empty — swallow so we don't accidentally submit.
          e.preventDefault();
          return;
        }
      }
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        submit();
      }
    },
    [menuOpen, items, safeIndex, choose, submit, closeMenu],
  );

  const activeModeLabel = SLASH_MACROS.find((m) => m.mode === activeMode)?.label;
  const resolvedPlaceholder =
    placeholder ??
    (activeMode
      ? t("lumilio.input.modePrompt", {
          defaultValue: "Continue in {{mode}} mode…",
          mode: activeModeLabel ?? activeMode,
        })
      : t("lumilio.input.prompt"));

  const menuHeader =
    menu.kind === "command"
      ? t("lumilio.mention.quickActions", "Quick Actions")
      : menu.kind === "entity"
        ? mentionTypes.find((m) => m.type === menu.activeType)?.label
        : t("lumilio.mention.mention", "Mention");

  const canSend = !disabled && !isGenerating && value.trim().length > 0;

  return (
    <div className="relative">
      {menuOpen && (
        <div className="absolute bottom-full left-0 z-dropdown mb-2 w-full overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-2xl shadow-base-content/10">
          <div className="px-3 pb-1 pt-2.5 text-[11px] font-semibold uppercase tracking-wider text-base-content/45">
            {menuHeader}
          </div>
          {items.length === 0 && (
            <div className="px-3 py-4 text-center text-sm text-base-content/45">
              {t("lumilio.mention.noResults", "No results")}
            </div>
          )}
          {items.length > 0 && (
            <ul className="max-h-64 overflow-y-auto px-1.5 pb-1.5">
              {items.map((item, index) => {
                const active = index === safeIndex;
                const style =
                  item.kind === "macro"
                    ? { icon: <Sparkles size={14} />, chip: MACRO_CHIP }
                    : item.kind === "type"
                      ? TYPE_STYLE[item.type]
                      : TYPE_STYLE[item.entity.type];
                const key =
                  item.kind === "type"
                    ? `t:${item.type}`
                    : item.kind === "entity"
                      ? `e:${item.entity.type}:${item.entity.id}`
                      : `c:${item.macro.id}`;
                const label =
                  item.kind === "type"
                    ? item.label
                    : item.kind === "entity"
                      ? item.entity.label
                      : item.macro.label;
                const desc = item.kind === "macro" ? item.macro.description : null;
                return (
                  <li key={key}>
                    <button
                      type="button"
                      className={`flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left transition-colors ${
                        active ? "bg-base-200" : "hover:bg-base-200/60"
                      }`}
                      onMouseEnter={() => setSelectedIndex(index)}
                      onMouseDown={(e) => {
                        e.preventDefault();
                        choose(item);
                      }}
                    >
                      <span
                        className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${style.chip}`}
                      >
                        {style.icon}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block truncate text-sm font-medium text-base-content">
                          {label}
                        </span>
                        {desc && (
                          <span className="block truncate text-xs text-base-content/50">
                            {desc}
                          </span>
                        )}
                      </span>
                      {item.kind === "type" && (
                        <ChevronRight size={15} className="shrink-0 text-base-content/30" />
                      )}
                    </button>
                  </li>
                );
              })}
            </ul>
          )}
          <div className="flex items-center gap-3 border-t border-base-300 bg-base-200/40 px-3 py-1.5 text-[11px] text-base-content/45">
            <span className="flex items-center gap-1">
              <kbd className="kbd kbd-xs">↑</kbd>
              <kbd className="kbd kbd-xs">↓</kbd>
              {t("lumilio.mention.navigate", "Navigate")}
            </span>
            <span className="flex items-center gap-1">
              <kbd className="kbd kbd-xs">↵</kbd>
              {t("lumilio.mention.select", "Select")}
            </span>
            <span className="ml-auto flex items-center gap-1">
              <kbd className="kbd kbd-xs">esc</kbd>
            </span>
          </div>
        </div>
      )}

      <div
        className={`overflow-hidden rounded-box border bg-base-100 transition-colors focus-within:border-primary ${
          menuOpen ? "border-primary/50" : "border-base-300"
        }`}
      >
        <textarea
          ref={textareaRef}
          rows={1}
          value={value}
          disabled={disabled}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onClick={syncMenu}
          onKeyUp={(e) => {
            if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) syncMenu();
          }}
          placeholder={resolvedPlaceholder}
          className="block max-h-[140px] min-h-[2.5rem] w-full resize-none bg-transparent px-3.5 pt-3 pb-1 text-sm leading-relaxed outline-none placeholder:text-base-content/40 disabled:opacity-60"
        />
        <div className="flex items-center gap-0.5 px-2 pb-2 pt-0.5">
          <button
            type="button"
            className={`btn btn-ghost btn-sm btn-circle ${
              activeMode ? "text-primary" : "text-base-content/45 hover:text-base-content"
            }`}
            disabled={disabled}
            aria-pressed={Boolean(activeMode)}
            title={t("lumilio.mention.quickActions", "Quick Actions")}
            onClick={() => openTrigger("/")}
          >
            <Sparkles size={17} strokeWidth={1.8} />
          </button>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle text-base-content/45 hover:text-base-content"
            disabled={disabled}
            title={t("lumilio.mention.mention", "Mention")}
            onClick={() => openTrigger("@")}
          >
            <AtSign size={17} strokeWidth={1.8} />
          </button>
          <button
            type="button"
            className="btn btn-primary btn-sm btn-circle ml-auto"
            aria-label={t("lumilio.input.send", "Send")}
            disabled={!canSend}
            onClick={submit}
          >
            {isGenerating ? (
              <span className="loading loading-spinner loading-xs" />
            ) : (
              <Send size={16} strokeWidth={1.8} />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
