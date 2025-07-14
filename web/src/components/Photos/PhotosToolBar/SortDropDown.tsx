export default function SortDropDown() {
  return (
    <div className="dropdown">
      <div tabIndex={0} role="button" className="btn btn-ghost m-1">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
          strokeWidth={1.5}
          stroke="currentColor"
          className="size-5"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
          />
        </svg>
      </div>
      <ul
        tabIndex={0}
        className="dropdown-content menu bg-base-200 rounded-box z-[1] w-52 p-2 shadow"
      >
        <li>
          <a>Group by Date</a>
        </li>
        <li>
          <a>Group by Size</a>
        </li>
        <li>
          <a>Group by Type</a>
        </li>
      </ul>
    </div>
  );
}
