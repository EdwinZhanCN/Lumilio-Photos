import type { CSSProperties, KeyboardEvent } from "react";
import { Check, Plus } from "lucide-react";

export interface TagPickerItem {
  id: string | number;
  name: string;
}

interface TagPickerMenuProps {
  query: string;
  onQueryChange: (q: string) => void;
  /** Already-selected items, rendered checked at the top. */
  checked: TagPickerItem[];
  /** Unselected library matches, rendered unchecked. */
  suggestions: TagPickerItem[];
  onToggleChecked: (item: TagPickerItem) => void;
  onSelectSuggestion: (item: TagPickerItem) => void;
  /** Optional "create new tag" row (omitted in filter mode). */
  showCreate?: boolean;
  createLabel?: string;
  createName?: string;
  onCreate?: () => void;
  loading?: boolean;
  placeholder: string;
  loadingText: string;
  noResultsText: string;
  autoFocus?: boolean;
  onKeyDown?: (e: KeyboardEvent<HTMLInputElement>) => void;
  /** Disable the search input and all tag rows (e.g. when the section toggle is off). */
  disabled?: boolean;
  className?: string;
  style?: CSSProperties;
}

/**
 * Shared Linear-style tag picker: a search box over a checkable list of tags.
 * Used both for editing an asset's tags (with "create") and for filtering the
 * gallery by tags (multi-select, no "create"). Purely presentational — the
 * parent owns the data and decides what checking/unchecking means.
 */
export default function TagPickerMenu({
  query,
  onQueryChange,
  checked,
  suggestions,
  onToggleChecked,
  onSelectSuggestion,
  showCreate = false,
  createLabel,
  createName,
  onCreate,
  loading = false,
  placeholder,
  loadingText,
  noResultsText,
  autoFocus = false,
  onKeyDown,
  disabled = false,
  className = "",
  style,
}: TagPickerMenuProps) {
  const isEmpty = !showCreate && suggestions.length === 0 && checked.length === 0;

  return (
    <div
      style={style}
      className={`flex flex-col rounded-box border border-base-300 bg-base-100 overflow-hidden font-sans ${className}`}
    >
      <div className="p-2 border-b border-base-200">
        <input
          type="text"
          autoFocus={autoFocus}
          value={query}
          placeholder={placeholder}
          className="input input-xs w-full"
          disabled={disabled}
          onChange={(e) => onQueryChange(e.target.value)}
          onKeyDown={onKeyDown}
        />
      </div>

      <ul className="menu menu-sm flex-nowrap overflow-y-auto custom-scrollbar p-1">
        {checked.map((item) => (
          <li key={`checked-${item.id}`}>
            <button
              type="button"
              className="flex items-center gap-2"
              disabled={disabled}
              onClick={() => onToggleChecked(item)}
            >
              <span className="flex items-center justify-center w-4 h-4 shrink-0 rounded bg-primary text-primary-content">
                <Check className="w-3 h-3" />
              </span>
              <span className="min-w-0 flex-1 truncate text-left">{item.name}</span>
            </button>
          </li>
        ))}

        {checked.length > 0 && (suggestions.length > 0 || showCreate) && (
          <li className="menu-title px-0 py-1">
            <span className="h-px bg-base-200" />
          </li>
        )}

        {suggestions.map((item) => (
          <li key={`suggest-${item.id}`}>
            <button
              type="button"
              className="flex items-center gap-2"
              disabled={disabled}
              onClick={() => onSelectSuggestion(item)}
            >
              <span className="w-4 h-4 shrink-0 rounded border border-base-content/30" />
              <span className="min-w-0 flex-1 truncate text-left">{item.name}</span>
            </button>
          </li>
        ))}

        {showCreate && onCreate && (
          <li>
            <button type="button" className="flex items-center gap-2" onClick={onCreate}>
              <Plus className="w-4 h-4 shrink-0" />
              <span className="min-w-0 flex-1 truncate text-left">
                {createLabel}
                <span className="opacity-50">{` "${createName}"`}</span>
              </span>
            </button>
          </li>
        )}

        {isEmpty && (
          <li className="px-3 py-2 text-xs text-base-content/40 pointer-events-none">
            {loading ? loadingText : noResultsText}
          </li>
        )}
      </ul>
    </div>
  );
}
