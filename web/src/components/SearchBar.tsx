import { MagnifyingGlassIcon } from "@heroicons/react/24/outline/index.js";
import { useState, useRef, useEffect, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import { SearchAssetsParams } from "@/services/assetsService";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";

interface SearchBarProps {
  onSearchResults?: (results: Asset[]) => void;
  onSearchError?: (error: string) => void;
}

export default function SearchBar({
  onSearchResults,
  onSearchError,
}: SearchBarProps = {}) {
  const { t } = useI18n();
  const { setSearchQuery, performAdvancedSearch, clearSearch } =
    useAssetsContext();

  const [searchText, setSearchText] = useState("");
  const [semanticSearch, setSemanticSearch] = useState(false);
  const [isSearching, setIsSearching] = useState(false);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  // Debounced search function
  const performSearch = useCallback(
    async (query: string, isSemanticSearch: boolean) => {
      if (!query.trim()) {
        setSearchQuery(""); // Clear search in context
        return;
      }

      setIsSearching(true);
      try {
        if (isSemanticSearch) {
          // Use advanced search API for semantic search
          const searchParams: SearchAssetsParams = {
            query: query.trim(),
            search_type: "semantic",
            limit: 50,
            offset: 0,
          };

          performAdvancedSearch(searchParams);
          onSearchResults?.([]); // Results will be handled by context
        } else {
          // Use simple filename search via context for backward compatibility
          setSearchQuery(query.trim());
        }
      } catch (error) {
        console.error("Search error:", error);
        const errorMessage =
          error instanceof Error ? error.message : "Search failed";
        onSearchError?.(errorMessage);

        // Fallback to simple text search in context
        setSearchQuery(query.trim());
      } finally {
        setIsSearching(false);
      }
    },
    [onSearchResults, onSearchError, setSearchQuery],
  );

  // Debounce search execution
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchText || searchText === "") {
        performSearch(searchText, semanticSearch);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchText, semanticSearch, performSearch]);

  useEffect(() => {
    if (active) {
      // focus the input when becoming active
      inputRef.current?.focus();
    }
  }, [active]);

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      performSearch(searchText, semanticSearch);
    }
  };

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
                  clearSearch?.();
                }
                return next;
              })
            }
            title={active ? t("search.close") : t("search.open")}
            disabled={isSearching}
          >
            {!isSearching && <MagnifyingGlassIcon className="size-5" />}
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
                    : t("search.placeholder", {
                        defaultValue: "Search by filename...",
                      })
                }
                value={searchText}
                className="input input-sm input-bordered search-input pr-8"
                onChange={handleInputChange}
                onKeyPress={handleKeyPress}
                disabled={isSearching}
              />
              {isSearching && (
                <div className="absolute right-2 top-1/2 transform -translate-y-1/2">
                  <span className="loading loading-spinner loading-xs"></span>
                </div>
              )}
            </div>

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
                  checked={semanticSearch}
                  onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                    setSemanticSearch(e.target.checked)
                  }
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
                  className="lucide lucide-sparkles-icon lucide-sparkles"
                >
                  <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z" />
                  <path d="M20 2v4" />
                  <path d="M22 4h-4" />
                  <circle cx="4" cy="20" r="2" />
                </svg>
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
                  <path d="M11.017 2.814a1 1 0 0 1 1.966 0l1.051 5.558a2 2 0 0 0 1.594 1.594l5.558 1.051a1 1 0 0 1 0 1.966l-5.558 1.051a2 2 0 0 0-1.594 1.594l-1.051 5.558a1 1 0 0 1-1.966 0l-1.051-5.558a2 2 0 0 0-1.594-1.594l-5.558-1.051a1 1 0 0 1 0-1.966l5.558-1.051a2 2 0 0 0 1.594-1.594z" />
                  <path d="M20 2v4" />
                  <path d="M22 4h-4" />
                  <circle cx="4" cy="20" r="2" />
                </svg>
              </label>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
