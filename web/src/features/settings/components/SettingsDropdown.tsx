import { CheckIcon, ChevronDownIcon } from "lucide-react";

export interface SettingsDropdownOption<T extends string = string> {
  value: T;
  label: string;
  disabled?: boolean;
}

interface SettingsDropdownProps<T extends string = string> {
  id?: string;
  value: T;
  options: ReadonlyArray<SettingsDropdownOption<T>>;
  onChange: (value: T) => void;
  disabled?: boolean;
  ariaLabel?: string;
  className?: string;
  menuClassName?: string;
}

export function SettingsDropdown<T extends string = string>({
  id,
  value,
  options,
  onChange,
  disabled,
  ariaLabel,
  className = "w-40",
  menuClassName = "w-52",
}: SettingsDropdownProps<T>) {
  const selected = options.find((option) => option.value === value) ?? options[0];

  const closeDropdown = () => {
    requestAnimationFrame(() => {
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }
    });
  };

  return (
    <div className={`dropdown dropdown-end ${className}`}>
      <div
        id={id}
        tabIndex={disabled ? -1 : 0}
        role="button"
        aria-label={ariaLabel}
        aria-disabled={disabled}
        className={`btn btn-sm w-full justify-between border border-base-300 bg-base-100 px-3 font-medium hover:bg-base-200 ${
          disabled ? "btn-disabled" : ""
        }`}
      >
        <span className="truncate">{selected?.label ?? value}</span>
        <ChevronDownIcon className="size-3.5 shrink-0 text-base-content/50" />
      </div>
      {!disabled && (
        <ul
          tabIndex={-1}
          className={`dropdown-content menu menu-sm z-dropdown mt-1 rounded-box border border-base-300 bg-base-100 p-1 shadow-sm ${menuClassName}`}
        >
          {options.map((option) => (
            <li key={option.value}>
              <button
                type="button"
                disabled={option.disabled}
                className={option.value === value ? "active" : ""}
                onClick={() => {
                  onChange(option.value);
                  closeDropdown();
                }}
              >
                <span className="min-w-0 flex-1 truncate text-left">{option.label}</span>
                {option.value === value && <CheckIcon className="size-4 shrink-0" />}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
