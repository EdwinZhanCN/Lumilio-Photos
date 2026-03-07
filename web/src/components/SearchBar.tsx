import { MagnifyingGlassIcon } from "@heroicons/react/24/outline/index.js";
import { useState, useRef, useEffect, useCallback } from "react";
import { useI18n } from "@/lib/i18n";
import {
  useCurrentTab,
  useSearchQuery,
  useUIActions,
} from "@/features/assets/selectors";

export default function SearchBar() {
  const { t } = useI18n();
  const currentTab = useCurrentTab();
  const searchQuery = useSearchQuery();
  const { setSearchQuery, setGroupBy } = useUIActions();

  const [searchText, setSearchText] = useState("");
  const [isSearching, setIsSearching] = useState(false);
  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  // Debounced search function
  const performSearch = useCallback(
    async (query: string) => {
      if (!query.trim()) {
        setSearchQuery("");
        return;
      }

      setIsSearching(true);
      try {
        // Set search query
        setSearchQuery(query.trim());

        // Auto-switch to flat view for better search results display
        setGroupBy("flat");
      } catch (error) {
        console.error("Search error:", error);
      } finally {
        setIsSearching(false);
      }
    },
    [setSearchQuery, setGroupBy],
  );

  // Debounce search execution
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchText || searchText === "") {
        performSearch(searchText);
      }
    }, 300);

    return () => clearTimeout(timer);
  }, [searchText, performSearch]);

  useEffect(() => {
    const nextQuery = searchQuery.trim();
    setSearchText(nextQuery);
    setActive(nextQuery.length > 0);
  }, [searchQuery]);

  useEffect(() => {
    if (active) {
      // focus the input when becoming active
      inputRef.current?.focus();
    }
  }, [active]);

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      performSearch(searchText);
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
                  setSearchQuery("");
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
                    currentTab === "photos"
                      ? t("search.placeholderai", {
                          defaultValue: "Search photos, scenes, and text...",
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
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
