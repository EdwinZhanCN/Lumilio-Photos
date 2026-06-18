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
  Slash,
} from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import {
  createMentionSources,
  getEntitiesForType,
  type MentionEntity,
  type MentionPayload,
  type MentionType,
} from "../../mentions/mentionSources";
import { SLASH_MACROS, type SlashMacro } from "../../slash/slashMacros";

const MAX_TEXTAREA_HEIGHT = 140;

const TYPE_ICON: Record<MentionType, ReactNode> = {
  person: <User size={15} />,
  album: <FolderOpen size={15} />,
  pin: <Pin size={15} />,
  camera: <Camera size={15} />,
  lens: <Aperture size={15} />,
};

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

interface MentionInputProps {
  isGenerating: boolean;
  disabled?: boolean;
  placeholder?: string;
  onSubmit: (query: string, mentions: MentionPayload[]) => void;
}

export function MentionInput({
  isGenerating,
  disabled = false,
  placeholder,
  onSubmit,
}: MentionInputProps) {
  const { t } = useI18n();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const pendingCaretRef = useRef<number | null>(null);

  const [value, setValue] = useState("");
  const [mentions, setMentions] = useState<MentionPayload[]>([]);
  const [menu, setMenu] = useState<Menu>({ kind: "idle" });
  const [selectedIndex, setSelectedIndex] = useState(0);

  const peopleQuery = $api.useQuery("get", "/api/v1/people", {
    params: { query: { limit: 200, offset: 0 } },
  });
  const albumsQuery = $api.useQuery("get", "/api/v1/albums", {
    params: { query: { limit: 100, offset: 0 } },
  });
  const pinsQuery = $api.useQuery("get", "/api/v1/agent/pins");
  const filterOptionsQuery = $api.useQuery(
    "get",
    "/api/v1/assets/filter-options",
  );

  const sources = useMemo(
    () =>
      createMentionSources({
        people: peopleQuery.data?.people ?? [],
        albums: albumsQuery.data?.albums ?? [],
        pins: pinsQuery.data ?? [],
        cameras: filterOptionsQuery.data?.camera_models ?? [],
        lenses: filterOptionsQuery.data?.lenses ?? [],
      }),
    [
      peopleQuery.data,
      albumsQuery.data,
      pinsQuery.data,
      filterOptionsQuery.data,
    ],
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
      return getEntitiesForType(sources, menu.activeType, menu.query).map(
        (entity) => ({ kind: "entity" as const, entity }),
      );
    }
    if (menu.kind === "command") {
      const q = menu.query.toLowerCase();
      return SLASH_MACROS.filter(
        (m) => m.label.toLowerCase().includes(q) || m.id.includes(q),
      ).map((macro) => ({ kind: "macro" as const, macro }));
    }
    return [];
  }, [menu, mentionTypes, sources]);

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

      // Second level (entity) or command: replace the trigger token with the
      // chosen label (entity) or the expanded template (command).
      const start = menu.start;
      const caret = textareaRef.current?.selectionStart ?? value.length;
      const before = value.slice(0, start);
      const after = value.slice(caret);
      const inserted =
        item.kind === "entity"
          ? `@${item.entity.label} `
          : `${item.macro.template} `;
      setValue(before + inserted + after);
      pendingCaretRef.current = before.length + inserted.length;
      if (item.kind === "entity") {
        setMentions((prev) =>
          prev.some(
            (m) => m.id === item.entity.id && m.type === item.entity.type,
          )
            ? prev
            : [...prev, { type: item.entity.type, id: item.entity.id, label: item.entity.label }],
        );
      }
      setMenu({ kind: "idle" });
    },
    [menu, value],
  );

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (menuOpen) {
        if (e.key === "Escape") {
          e.preventDefault();
          setMenu({ kind: "idle" });
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
    [menuOpen, items, safeIndex, choose, submit],
  );

  const menuHeader =
    menu.kind === "command"
      ? t("lumilio.mention.commands", "Commands")
      : menu.kind === "entity"
        ? mentionTypes.find((m) => m.type === menu.activeType)?.label
        : t("lumilio.mention.mention", "Mention");

  return (
    <div className="relative">
      {menuOpen && (
        <div className="absolute bottom-full left-0 z-50 mb-1.5 w-full overflow-hidden rounded-box border border-base-300 bg-base-100 shadow-xl">
          <div className="flex justify-between border-b border-base-300 bg-base-200 px-3 py-1.5 text-xs font-semibold uppercase tracking-wider text-base-content/60">
            <span>{menuHeader}</span>
            <span className="font-mono">Tab</span>
          </div>
          {items.length === 0 && (
            <div className="px-3 py-3 text-center text-sm text-base-content/50">
              {t("lumilio.mention.noResults", "No results")}
            </div>
          )}
          <ul className="max-h-56 overflow-y-auto py-1">
            {items.map((item, index) => {
              const active = index === safeIndex;
              const key =
                item.kind === "type"
                  ? `t:${item.type}`
                  : item.kind === "entity"
                    ? `e:${item.entity.type}:${item.entity.id}`
                    : `c:${item.macro.id}`;
              const icon =
                item.kind === "macro" ? (
                  <Slash size={15} />
                ) : item.kind === "type" ? (
                  TYPE_ICON[item.type]
                ) : (
                  TYPE_ICON[item.entity.type]
                );
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
                    className={`flex w-full items-center gap-2.5 px-3 py-1.5 text-left text-sm ${
                      active
                        ? "bg-primary text-primary-content"
                        : "text-base-content hover:bg-base-200"
                    }`}
                    onMouseEnter={() => setSelectedIndex(index)}
                    onMouseDown={(e) => {
                      e.preventDefault();
                      choose(item);
                    }}
                  >
                    <span className={active ? "" : "text-base-content/50"}>
                      {icon}
                    </span>
                    <span className="flex-1 truncate font-medium">{label}</span>
                    {item.kind === "type" && (
                      <span className="opacity-50">›</span>
                    )}
                    {desc && (
                      <span className="ml-2 truncate text-xs opacity-60">
                        {desc}
                      </span>
                    )}
                  </button>
                </li>
              );
            })}
          </ul>
        </div>
      )}

      <div className="flex items-end gap-2 rounded-box border border-base-300 bg-base-100 px-2.5 py-1.5 focus-within:ring-2 focus-within:ring-primary">
        <textarea
          ref={textareaRef}
          rows={1}
          value={value}
          disabled={disabled}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onClick={syncMenu}
          onKeyUp={(e) => {
            if (["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key))
              syncMenu();
          }}
          placeholder={placeholder ?? t("lumilio.input.prompt")}
          className="max-h-[140px] min-h-9 flex-1 resize-none bg-transparent py-1.5 text-sm leading-relaxed outline-none placeholder:text-base-content/40 disabled:opacity-60"
        />
        <button
          type="button"
          className="btn btn-primary btn-circle btn-sm shrink-0"
          aria-label={t("lumilio.input.send", "Send")}
          disabled={disabled || isGenerating || value.trim().length === 0}
          onClick={submit}
        >
          <Send size={16} strokeWidth={1.8} />
        </button>
      </div>
    </div>
  );
}
