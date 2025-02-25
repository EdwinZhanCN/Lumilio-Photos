import React, {useState} from "react";
import { Link } from "react-router-dom";

function NavBar() {
    const [searchText, setSearchText] = useState("");
    const [searchOption, setSearchOption] = useState(null);

    const handleOptionSelect = (option) => {
        setSearchOption(option);
        setSearchText(option + ":");
    };

    const handleInputChange = (e) => {
        setSearchText(e.target.value);
    };

    return (
        <div className="navbar bg-base-100 px-4 py-2">
            {/* Branding */}
            <div className="flex-none">
                <Link className="btn btn-ghost text-xl" to="/">
                    <svg className="w-6 h-6" viewBox="0 0 63 63" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <rect width="49.2746" height="51.5858" rx="8" transform="matrix(0.987518 -0.157508 0.143399 0.989665 0 7.76113)" fill="#e7e7e7"/>
                        <rect width="49.295" height="51.5663" rx="8" transform="matrix(0.982949 0.183877 -0.167538 0.985866 14.5455 3.09835)" fill="#b3b3b3"/>
                        <path d="M16.0789 36.6541L11.8722 38.661C9.56934 39.7596 7.94936 41.9132 7.51848 44.4487L7.24654 46.0489C6.50631 50.4047 9.4269 54.5944 13.7698 55.4068L48.1684 61.8416C51.5883 62.4814 54.8332 60.2194 55.4161 56.7894C55.7021 55.1067 55.295 53.3687 54.2882 51.9731L50.1622 46.2542C47.9899 43.2432 44.0775 42.0807 40.6841 43.4379L37.4869 44.7167C34.1519 46.0506 30.3069 44.9523 28.1106 42.0384L25.9182 39.1297C23.6064 36.0625 19.49 35.0268 16.0789 36.6541Z" fill="#F3F3F3"/>
                        <ellipse cx="8.38015" cy="8.76627" rx="8.38015" ry="8.76627" transform="matrix(0.982949 0.183877 -0.167538 0.985866 37.3904 18.5901)" fill="#727272"/>
                    </svg>
                    Lumina
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
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                                            <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM12.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0ZM18.75 12a.75.75 0 1 1-1.5 0 .75.75 0 0 1 1.5 0Z" />
                                        </svg>
                                        Name
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('tag')}>
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                                            <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 8.25h15m-16.5 7.5h15m-1.8-13.5-3.9 19.5m-2.1-19.5-3.9 19.5" />
                                        </svg>
                                        Tag
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('date')}>
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                                            <path strokeLinecap="round" strokeLinejoin="round" d="M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5" />
                                        </svg>
                                        Date
                                    </a>
                                </li>
                                <li>
                                    <a onClick={() => handleOptionSelect('type')}>
                                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                                            <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                                        </svg>
                                        File Type
                                    </a>
                                </li>
                            </ul>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}

export default NavBar;