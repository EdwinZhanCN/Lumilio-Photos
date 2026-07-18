import { useRef, useState } from "react";
import { Search, X } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { useSearchQuery, useUIActions } from "../../selectors";

interface SearchFABProps {
  className?: string;
}

export function SearchFAB({ className }: SearchFABProps) {
  const { t } = useI18n();
  const searchQuery = useSearchQuery();
  const { setSearchQuery } = useUIActions();

  const [localValue, setLocalValue] = useState(searchQuery);
  const inputRef = useRef<HTMLInputElement>(null);

  // Keep the fab expanded (override :focus-within) while there's search text
  const hasSearch = localValue.length > 0;

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setLocalValue(val);
    setSearchQuery(val);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setSearchQuery(localValue);
  };

  // Clear input text only — keep fab open so user can retype
  const handleClearInput = () => {
    setLocalValue("");
    setSearchQuery("");
    inputRef.current?.focus();
  };

  // Close the entire fab: clear search state.
  // fab-close is not focusable, so clicking it also drops :focus-within.
  const handleClose = () => {
    setLocalValue("");
    setSearchQuery("");
  };

  return (
    <>
      {/* Backdrop — visible while search input is shown */}
      {hasSearch && (
        <div
          aria-hidden="true"
          className="fixed bottom-0 right-0 z-30 h-[40vh] w-[28rem] pointer-events-none"
          style={{
            backdropFilter: "blur(4px)",
            WebkitBackdropFilter: "blur(4px)",
            maskImage: `
              linear-gradient(to bottom, transparent 0%, black 45%),
              linear-gradient(to right, transparent 0%, black 35%)
            `,
            WebkitMaskImage: `
              linear-gradient(to bottom, transparent 0%, black 45%),
              linear-gradient(to right, transparent 0%, black 35%)
            `,
            maskComposite: "intersect",
            WebkitMaskComposite: "source-in",
          }}
        />
      )}

      {/*
        daisyUI fab with fab-close pattern.
        - Trigger (tabIndex div) opens via :focus-within
        - fab-close (not focusable) closes by dropping :focus-within
        - fab-open class keeps children visible when browsing search results
      */}
      <div
        className={`fab fixed bottom-6 right-4 z-40 flex items-center gap-2 ${hasSearch ? "fab-open" : ""} ${className ?? ""}`}
        style={{ flexDirection: "row-reverse" }}
      >
        {/* ── Trigger button (focusable, first child) ── */}
        <div
          tabIndex={0}
          role="button"
          aria-label="Search photos"
          onClick={() => inputRef.current?.focus()}
          className="btn btn-circle btn-soft btn-lg btn-info"
        >
          <Search className="size-5" />
        </div>

        {/* ── Close button (not focusable — clicking drops :focus-within) ── */}
        <div className="fab-close" onClick={handleClose}>
          <span className="btn btn-circle btn-lg btn-error">
            <X className="size-5" />
          </span>
        </div>

        {/* ── Search input ── */}
        <form onSubmit={handleSubmit} role="search">
          <div className="relative flex items-center">
            <input
              ref={inputRef}
              id="gallery-search-input"
              type="search"
              role="searchbox"
              aria-label="Search photos"
              value={localValue}
              onChange={handleChange}
              placeholder={t("assets.searchPlaceholder", "Search assets...")}
              className="input input-bordered rounded-full w-72 max-w-[calc(100vw-6rem)] bg-base-100 shadow-md text-sm"
            />
            {localValue && (
              <button
                type="button"
                aria-label="Clear search"
                onClick={handleClearInput}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-base-content/40 hover:text-base-content"
              >
                <X className="size-4" />
              </button>
            )}
          </div>
        </form>
      </div>
    </>
  );
}
