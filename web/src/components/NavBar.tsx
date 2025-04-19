import React, {useState} from "react";
import { Link } from "react-router-dom";
import {CalendarIcon, DocumentIcon, LanguageIcon, TagIcon} from "@heroicons/react/24/outline/index.js";


function NavBar() {
    const [searchText, setSearchText] = useState("");
    const [searchOption, setSearchOption] = useState<string>("");
    const [isDarkMode, setIsDarkMode] = useState(localStorage.getItem("theme") === "dark");

    const handleOptionSelect = (option:string) => {
        setSearchOption(option);
        setSearchText(searchOption + ":");
    };

    const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        setSearchText(e.target.value);
    };

    return (
        <div className="navbar bg-base-100 px-4 py-2">
            {/* Branding */}
            <div className="flex-none">
                <Link className="btn btn-ghost text-xl" to="/">
                    <img src={"/logo.png"} className="size-6 bg-contain object-contain" alt="Lumilio Logo"/>
                    Lumilio
                </Link>
            </div>
            {/* Center search bar */}
            <div className="flex-1">
                <div className="flex justify-center">
                    <div className="flex flex-row items-center gap-3 w-full max-w-lg">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="size-6">
                            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
                        </svg>
                        <input
                            type="text"
                            placeholder={"Search"}
                            value={searchText ? searchText : ""}
                            className="input input-bordered"
                            onChange={handleInputChange}
                        />
                        <div className="dropdown dropdown-hover">
                            <div tabIndex={0} role="button" className="btn btn-circle m-1">
                                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth="1.5" stroke="currentColor" className="w-5 h-5">
                                    <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 15 12 18.75 15.75 15m-7.5-6L12 5.25 15.75 9" />
                                </svg>
                            </div>
                            <ul tabIndex={0} className="dropdown-content menu bg-base-100 rounded-box z-[1] w-52 p-2 shadow">
                                <li>
                                    <a onClick={() => handleOptionSelect('name')}>
                                        <LanguageIcon className="size-4"/>
                                        Name
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('tag')}>
                                        <TagIcon className="size-4"/>
                                        Tag
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('date')}>
                                        <CalendarIcon className="size-4"/>
                                        Date
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('type')}>
                                        <DocumentIcon className="size-4"/>
                                        File Type
                                    </a>
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
            {/* Theme Controller */}
            <label className="swap swap-rotate">
                {/* this hidden checkbox controls the state */}
                <input type="checkbox"
                       className="theme-controller"
                       value="dark"
                       checked={isDarkMode}
                       onChange={(e) => {
                           const newTheme = e.target.checked ? "dark" : "light";
                           localStorage.setItem("theme", newTheme);
                           document.documentElement.setAttribute("data-theme", newTheme);
                           setIsDarkMode(e.target.checked);
                       }}
                />

                {/* sun icon */}
                <svg
                    className="swap-off h-6 w-6 fill-current"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                >
                    <path
                        d="M5.64,17l-.71.71a1,1,0,0,0,0,1.41,1,1,0,0,0,1.41,0l.71-.71A1,1,0,0,0,5.64,17ZM5,12a1,1,0,0,0-1-1H3a1,1,0,0,0,0,2H4A1,1,0,0,0,5,12Zm7-7a1,1,0,0,0,1-1V3a1,1,0,0,0-2,0V4A1,1,0,0,0,12,5ZM5.64,7.05a1,1,0,0,0,.7.29,1,1,0,0,0,.71-.29,1,1,0,0,0,0-1.41l-.71-.71A1,1,0,0,0,4.93,6.34Zm12,.29a1,1,0,0,0,.7-.29l.71-.71a1,1,0,1,0-1.41-1.41L17,5.64a1,1,0,0,0,0,1.41A1,1,0,0,0,17.66,7.34ZM21,11H20a1,1,0,0,0,0,2h1a1,1,0,0,0,0-2Zm-9,8a1,1,0,0,0-1,1v1a1,1,0,0,0,2,0V20A1,1,0,0,0,12,19ZM18.36,17A1,1,0,0,0,17,18.36l.71.71a1,1,0,0,0,1.41,0,1,1,0,0,0,0-1.41ZM12,6.5A5.5,5.5,0,1,0,17.5,12,5.51,5.51,0,0,0,12,6.5Zm0,9A3.5,3.5,0,1,1,15.5,12,3.5,3.5,0,0,1,12,15.5Z" />
                </svg>

                {/* moon icon */}
                <svg
                    className="swap-on h-6 w-6 fill-current"
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24">

                    <path
                        d="M21.64,13a1,1,0,0,0-1.05-.14,8.05,8.05,0,0,1-3.37.73A8.15,8.15,0,0,1,9.08,5.49a8.59,8.59,0,0,1,.25-2A1,1,0,0,0,8,2.36,10.14,10.14,0,1,0,22,14.05,1,1,0,0,0,21.64,13Zm-9.5,6.69A8.14,8.14,0,0,1,7.08,5.22v.27A10.15,10.15,0,0,0,17.22,15.63a9.79,9.79,0,0,0,2.1-.22A8.11,8.11,0,0,1,12.14,19.73Z" />
                </svg>
            </label>
        </div>
    );
}

export default NavBar;