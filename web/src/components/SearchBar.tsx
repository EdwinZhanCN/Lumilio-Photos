import { MagnifyingGlassIcon } from "@heroicons/react/24/outline/index.js";
import { useState, useRef, useEffect } from "react";
import { useI18n } from "@/lib/i18n";

interface SearchBarProps {
  value?: string;
  onChange?: (query: string) => void;
  /**
   * Callback fired whenever the search UI (input) is activated or deactivated
   * Allows parent to react (e.g., auto-switch to flat view)
   */
  onActivationChange?: (active: boolean) => void;
  placeholder?: string;
  enableSemanticSearch?: boolean;
}

export default function SearchBar({
  value = "",
  onChange,
  onActivationChange,
  placeholder = "Search...",
  enableSemanticSearch = true,
}: SearchBarProps = {}) {
  const { t } = useI18n();

  const [searchText, setSearchText] = useState(value);
  const [semanticSearch, setSemanticSearch] = useState(false);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    setSearchText(newValue);
    onChange?.(newValue);
  };

  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Sync external value changes
  useEffect(() => {
    setSearchText(value);
  }, [value]);

  // Handle search execution - simplified for new architecture
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchText !== value) {
        onChange?.(searchText);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchText, value, onChange]);

  useEffect(() => {
    if (active) {
      // focus the input when becoming active
      inputRef.current?.focus();
    }
  }, [active]);

  // Notify parent about activation state changes (single source)
  useEffect(() => {
    onActivationChange?.(active);
  }, [active, onActivationChange]);

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      // Trigger immediate search on Enter
      onChange?.(searchText);
    }
  };

  return (
    <div className="flex-1">
      <div className="flex justify-center">
        <div className="flex flex-row items-center gap-3 w-full max-w-lg">
          <button
            className={`btn btn-sm btn-circle btn-soft btn-info ${active ? "btn-active" : ""}`}
            aria-pressed={active}
            onClick={() =>
              setActive((prev) => {
                const next = !prev;
                // Deactivating search: clear local search state
                if (!next) {
                  setSearchText("");
                  onChange?.("");
                }
                return next;
              })
            }
            title={active ? t("search.close") : t("search.open")}
          >
            <MagnifyingGlassIcon className="size-5" />
          </button>
          <div
            className={`search-controls flex flex-row items-center gap-3 ${active ? "visible" : "hidden"}`}
          >
            <div className="relative">
              <input
                ref={inputRef}
                type="text"
                placeholder={
                  semanticSearch
                    ? t("search.placeholderai", {
                        defaultValue: "Describe what you're looking for...",
                      })
                    : placeholder
                }
                value={searchText}
                className="input input-sm input-bordered search-input pr-8"
                onChange={handleInputChange}
                onKeyPress={handleKeyPress}
              />
            </div>

            {enableSemanticSearch && (
              <div
                className="tooltip tooltip-bottom"
                data-tip={
                  semanticSearch
                    ? t("search.option.semantic")
                    : t("search.option.filename")
                }
              >
                <label className="toggle animate-fade-in-x cursor-pointer">
                  <input
                    type="checkbox"
                    checked={semanticSearch && enableSemanticSearch}
                    onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                      setSemanticSearch(e.target.checked)
                    }
                    disabled={!enableSemanticSearch}
                  />
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-sparkles-icon lucide-sparkles"
                  >
                    <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0 1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z" />
                    <path d="M20 2v4" />
                    <path d="M22 4h-4" />
                    <circle cx="4" cy="20" r="2" />
                  </svg>
                </label>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
