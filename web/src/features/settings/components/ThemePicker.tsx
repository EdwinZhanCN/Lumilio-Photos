/**
 * ThemePicker — the daisyUI theme swatch grid. One instance renders one
 * mode (light or dark); the Appearance tab stacks two.
 */
import type React from "react";
import { type DaisyUIDarkThemeName, type DaisyUILightThemeName } from "@/lib/theme/daisyuiThemes";

export type ModeThemeName = DaisyUILightThemeName | DaisyUIDarkThemeName;

interface ThemePickerProps {
  title: string;
  description: string;
  activeBadgeLabel: string;
  isActiveMode: boolean;
  themes: readonly ModeThemeName[];
  selectedTheme: ModeThemeName;
  icon: React.ComponentType<React.ComponentProps<"svg">>;
  onSelect: (theme: ModeThemeName) => void;
}

export function ThemePicker({
  title,
  description,
  activeBadgeLabel,
  isActiveMode,
  themes,
  selectedTheme,
  icon: Icon,
  onSelect,
}: ThemePickerProps) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <Icon className="size-4 text-primary" />
            <h4 className="text-sm font-medium">{title}</h4>
          </div>
          <p className="text-sm text-base-content/60">{description}</p>
        </div>
        {isActiveMode ? (
          <span className="badge badge-primary badge-soft">{activeBadgeLabel}</span>
        ) : null}
      </div>

      <div className="rounded-box grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
        {themes.map((theme) => {
          const isSelected = selectedTheme === theme;

          return (
            <button
              key={theme}
              type="button"
              aria-pressed={isSelected}
              aria-label={`${title}: ${theme}`}
              className={[
                "border-base-content/20 hover:border-base-content/40 overflow-hidden rounded-lg border outline-2 outline-offset-2 transition",
                isSelected ? "border-base-content/40 outline-base-content" : "outline-transparent",
              ].join(" ")}
              onClick={() => onSelect(theme)}
            >
              <div
                className="bg-base-100 text-base-content w-full cursor-pointer font-sans"
                data-theme={theme}
              >
                <div className="grid grid-cols-5 grid-rows-3">
                  <div className="bg-base-200 col-start-1 row-span-2 row-start-1" />
                  <div className="bg-base-300 col-start-1 row-start-3" />
                  <div className="bg-base-100 col-span-4 col-start-2 row-span-3 row-start-1 flex flex-col gap-1 p-2 text-left">
                    <div className="font-bold">{theme}</div>
                    <div className="flex flex-wrap gap-1">
                      <div className="bg-primary flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-primary-content text-sm font-bold">A</div>
                      </div>
                      <div className="bg-secondary flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-secondary-content text-sm font-bold">A</div>
                      </div>
                      <div className="bg-accent flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-accent-content text-sm font-bold">A</div>
                      </div>
                      <div className="bg-neutral flex aspect-square w-5 items-center justify-center rounded lg:w-6">
                        <div className="text-neutral-content text-sm font-bold">A</div>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            </button>
          );
        })}
      </div>
    </div>
  );
}
