import { MagnifyingGlassIcon } from "@heroicons/react/24/outline/index.js";
import { useState, useRef, useEffect, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";

interface SearchBarProps {
  enableSemanticSearch?: boolean;
}

export default function SearchBar({
  enableSemanticSearch = false,
}: SearchBarProps) {
  const { t } = useI18n();
  const { state, dispatch } = useAssetsContext();

  const [searchText, setSearchText] = useState("");
  const [isSearching, setIsSearching] = useState(false);
  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const isSemanticMode = state.ui.searchMode === "semantic";

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  // Debounced search function
  const performSearch = useCallback(
    async (query: string, isSemanticSearch: boolean) => {
      if (!query.trim()) {
        dispatch({ type: "SET_SEARCH_QUERY", payload: "" });
        return;
      }

      setIsSearching(true);
      try {
        // Update search mode based on semantic toggle
        const searchMode = isSemanticSearch ? "semantic" : "filename";
        dispatch({ type: "SET_SEARCH_MODE", payload: searchMode });

        // Set search query
        dispatch({ type: "SET_SEARCH_QUERY", payload: query.trim() });

        // Auto-switch to flat view for better search results display
        dispatch({ type: "SET_GROUP_BY", payload: "flat" });
      } catch (error) {
        console.error("Search error:", error);
      } finally {
        setIsSearching(false);
      }
    },
    [dispatch],
  );

  // Debounce search execution
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchText || searchText === "") {
        performSearch(searchText, isSemanticMode);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchText, isSemanticMode, performSearch]);

  useEffect(() => {
    if (active) {
      // focus the input when becoming active
      inputRef.current?.focus();
    }
  }, [active]);

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      performSearch(searchText, isSemanticMode);
    }
  };

  const toggleSemanticSearch = useCallback(() => {
    const newMode = isSemanticMode ? "filename" : "semantic";
    dispatch({ type: "SET_SEARCH_MODE", payload: newMode });
  }, [isSemanticMode, dispatch]);

  return (
    <div className="flex-1">
      <div className="flex justify-center">
        <div className="flex flex-row items-center gap-3 w-full max-w-lg">
          <button
            className={`btn btn-sm btn-circle btn-soft btn-info ${active ? "btn-active" : ""} ${isSearching ? "loading" : ""}`}
            aria-pressed={active}
            onClick={() =>
              setActive((prev) => {
                const next = !prev;
                // Deactivating search: clear local and context search state
                if (!next) {
                  setSearchText("");
                  dispatch({ type: "SET_SEARCH_QUERY", payload: "" });
                }
                return next;
              })
            }
            title={active ? t("search.close") : t("search.open")}
            disabled={isSearching}
          >
            {!isSearching && <MagnifyingGlassIcon className="w-4 h-4" />}
          </button>
          {active && (
            <div
              className={`search-controls flex flex-row items-center gap-3 ${active ? "visible" : "hidden"}`}
            >
              <div className="relative">
                <input
                  ref={inputRef}
                  id="search-input"
                  name="search"
                  type="text"
                  placeholder={
                    isSemanticMode && enableSemanticSearch
                      ? t("search.placeholderai", {
                          defaultValue: "Describe what you're looking for...",
                        })
                      : t("search.placeholder", {
                          defaultValue: "Search by filename...",
                        })
                  }
                  value={searchText}
                  className="input input-sm input-bordered search-input pr-8"
                  onChange={handleInputChange}
                  onKeyDown={handleKeyPress}
                  disabled={isSearching}
                />
                {isSearching && (
                  <div className="absolute right-2 top-1/2 transform -translate-y-1/2">
                    <span className="loading loading-spinner loading-xs"></span>
                  </div>
                )}
              </div>

              {enableSemanticSearch && (
                <div
                  className="tooltip tooltip-bottom"
                  data-tip={
                    isSemanticMode
                      ? t("search.option.semantic")
                      : t("search.option.filename")
                  }
                >
                  <label className="toggle animate-fade-in-x cursor-pointer">
                    <input
                      type="checkbox"
                      checked={isSemanticMode}
                      onChange={toggleSemanticSearch}
                      disabled={isSearching}
                    />
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="lucide lucide-sparkles"
                    >
                      <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z" />
                      <path d="M20 2v4" />
                      <path d="M22 4h-4" />
                      <circle cx="4" cy="20" r="2" />
                    </svg>
                  </label>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
