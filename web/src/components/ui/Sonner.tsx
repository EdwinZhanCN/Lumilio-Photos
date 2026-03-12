import { useEffect, useState, type CSSProperties } from "react";
import { Toaster as Sonner, type ToasterProps } from "sonner";
import {
  CircleCheckIcon,
  InfoIcon,
  TriangleAlertIcon,
  OctagonXIcon,
  Loader2Icon,
} from "lucide-react";
import { isDarkDaisyUITheme } from "@/lib/theme/daisyuiThemes";

function resolveTheme(): ToasterProps["theme"] {
  if (typeof document === "undefined") return "light";

  const dataTheme = document.documentElement.getAttribute("data-theme");
  if (dataTheme) {
    return isDarkDaisyUITheme(dataTheme) ? "dark" : "light";
  }

  if (
    typeof window !== "undefined" &&
    typeof window.matchMedia === "function" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }

  return "light";
}

const Toaster = ({ ...props }: ToasterProps) => {
  const [theme, setTheme] = useState<ToasterProps["theme"]>(resolveTheme);

  useEffect(() => {
    const updateTheme = () => setTheme(resolveTheme());

    updateTheme();

    const observer = new MutationObserver(updateTheme);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    media.addEventListener("change", updateTheme);

    return () => {
      observer.disconnect();
      media.removeEventListener("change", updateTheme);
    };
  }, []);

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      closeButton={false}
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "hsl(var(--b1))",
          "--normal-text": "hsl(var(--bc))",
          "--normal-border": "hsl(var(--b3))",
          "--success-bg": "hsl(var(--su) / 0.12)",
          "--success-text": "hsl(var(--su))",
          "--success-border": "hsl(var(--su) / 0.35)",
          "--error-bg": "hsl(var(--er) / 0.12)",
          "--error-text": "hsl(var(--er))",
          "--error-border": "hsl(var(--er) / 0.35)",
          "--warning-bg": "hsl(var(--wa) / 0.12)",
          "--warning-text": "hsl(var(--wa))",
          "--warning-border": "hsl(var(--wa) / 0.35)",
          "--info-bg": "hsl(var(--in) / 0.12)",
          "--info-text": "hsl(var(--in))",
          "--info-border": "hsl(var(--in) / 0.35)",
          "--border-radius": "0.75rem",
        } as CSSProperties
      }
      toastOptions={{
        duration: 5000,
        classNames: {
          toast:
            "border bg-base-100 text-base-content shadow-lg backdrop-blur supports-[backdrop-filter]:bg-base-100/95",
          title: "text-sm font-medium",
          description: "text-xs text-base-content/70",
          actionButton: "btn btn-xs btn-primary",
          cancelButton: "btn btn-xs btn-ghost",
        },
      }}
      {...props}
    />
  );
};

export { Toaster };
