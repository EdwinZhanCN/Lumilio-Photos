import {
  CalendarIcon,
  DocumentIcon,
  LanguageIcon,
  TagIcon,
} from "@heroicons/react/24/outline/index.js";
import { LumenAvatar } from "@/features/lumen";
import { useState } from "react";
import { Link } from "react-router-dom";
import { useI18n } from "@/lib/i18n";

export default function SearchBar() {
  const { t } = useI18n();

  const [searchText, setSearchText] = useState("");
  const [searchOption, setSearchOption] = useState<string>("");
  const handleOptionSelect = (option: string) => {
    setSearchOption(option);
    setSearchText(searchOption + ":");
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchText(e.target.value);
  };
  return (
    <div className="flex-1">
      <div className="flex justify-center">
        <div className="flex flex-row items-center gap-3 w-full max-w-lg">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            strokeWidth={1.5}
            stroke="currentColor"
            className="size-6"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z"
            />
          </svg>
          <input
            type="text"
            placeholder={t("navbar.search.placeholder")}
            value={searchText ? searchText : ""}
            className="input input-bordered"
            onChange={handleInputChange}
          />
          <div className="dropdown dropdown-hover">
            <div tabIndex={0} role="button" className="btn btn-circle m-1">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                strokeWidth="1.5"
                stroke="currentColor"
                className="w-5 h-5"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M8.25 15 12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9"
                />
              </svg>
            </div>
            <ul
              tabIndex={0}
              className="dropdown-content menu bg-base-100 rounded-box z-[1] w-52 p-2 shadow"
            >
              <li>
                <a onClick={() => handleOptionSelect("name")}>
                  <LanguageIcon className="size-4" />
                  {t("navbar.search.option.name")}
                </a>
              </li>
              <li>
                <a onClick={() => handleOptionSelect("tag")}>
                  <TagIcon className="size-4" />
                  {t("navbar.search.option.tag")}
                </a>
              </li>
              <li>
                <a onClick={() => handleOptionSelect("date")}>
                  <CalendarIcon className="size-4" />
                  {t("navbar.search.option.date")}
                </a>
              </li>
              <li>
                <a onClick={() => handleOptionSelect("type")}>
                  <DocumentIcon className="size-4" />
                  {t("navbar.search.option.type")}
                </a>
              </li>
            </ul>
          </div>
          <div>
            {(() => {
              const [start, setStart] = useState(false);
              return (
                <Link to={"/lumen"}>
                  <div
                    onMouseEnter={() => setStart(true)}
                    onMouseLeave={() => setStart(false)}
                  >
                    <LumenAvatar
                      className="mb-2 pointer-cursor"
                      size={0.2}
                      start={start}
                    />
                  </div>
                </Link>
              );
            })()}
          </div>
        </div>
      </div>
    </div>
  );
}
