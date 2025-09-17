import { MagnifyingGlassIcon } from "@heroicons/react/24/outline/index.js";
import { useState, useRef, useEffect } from "react";
import { useI18n } from "@/lib/i18n";

export default function SearchBar() {
  const { t } = useI18n();

  const [searchText, setSearchText] = useState("");
  const [semanticSearch, setSemanticSearch] = useState(false);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };

  const [active, setActive] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (active) {
      // focus the input when becoming active
      inputRef.current?.focus();
    }
  }, [active]);

  return (
    <div className="flex-1">
      <div className="flex justify-center">
        <div className="flex flex-row items-center gap-3 w-full max-w-lg">
          <button
            className={`btn btn-sm btn-circle btn-soft btn-info ${active ? "btn-active" : ""}`}
            aria-pressed={active}
            onClick={() => setActive((v) => !v)}
            title={active ? t("navbar.search.close") : t("navbar.search.open")}
          >
            <MagnifyingGlassIcon className="size-5" />
          </button>
          <div
            className={`search-controls flex flex-row items-center gap-3 ${active ? "visible" : "hidden"}`}
          >
            <input
              ref={inputRef}
              type="text"
              placeholder={t("navbar.search.placeholder")}
              value={searchText ? searchText : ""}
              className="input input-sm input-bordered search-input"
              onChange={handleInputChange}
            />

            <label className="toggle animate-fade-in-x">
              <input
                type="checkbox"
                checked={semanticSearch}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                  setSemanticSearch(e.target.checked)
                }
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
  );
}
