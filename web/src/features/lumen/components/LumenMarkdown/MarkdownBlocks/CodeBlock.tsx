import React, { useState, useEffect } from "react";
import { Prism as SyntaxHighlighter } from "react-syntax-highlighter";
import {
  oneDark,
  oneLight,
} from "react-syntax-highlighter/dist/cjs/styles/prism";

interface CodeBlockProps {
  inline?: boolean;
  className?: string;
  children?: React.ReactNode;
  [key: string]: any;
}

export const CodeBlock: React.FC<CodeBlockProps> = ({
  inline,
  className,
  children,
  ...props
}) => {
  const [copied, setCopied] = useState(false);
  const [isDark, setIsDark] = useState(false);
  const match = /language-(\w+)/.exec(className || "");
  const language = match ? match[1] : "";
  const code = String(children).replace(/\n$/, "");

  // Detect theme changes
  useEffect(() => {
    const checkTheme = () => {
      const html = document.documentElement;
      const theme = html.getAttribute("data-theme");
      const isDarkTheme =
        theme === "macchiato" ||
        (!theme &&
          window.matchMedia("(prefers-color-scheme: macchiato)").matches);
      setIsDark(isDarkTheme);
    };

    checkTheme();

    // Listen for theme changes
    const observer = new MutationObserver(checkTheme);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    // Listen for system theme changes
    const mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
    mediaQuery.addEventListener("change", checkTheme);

    return () => {
      observer.disconnect();
      mediaQuery.removeEventListener("change", checkTheme);
    };
  }, []);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (inline) {
    return (
      <code
        className="bg-base-200 px-1 py-0.5 rounded text-sm font-mono"
        {...props}
      >
        {children}
      </code>
    );
  }

  return (
    <div className="relative group my-4">
      <div className="flex items-center justify-between bg-base-300 px-4 py-2 rounded-t-lg">
        <span className="text-sm text-base-content/70 font-medium">
          {language || "text"}
        </span>
        <button
          onClick={handleCopy}
          className="text-base-content/60 hover:text-base-content transition-colors duration-200 text-sm"
        >
          {copied ? "✓ Copied" : "Copy"}
        </button>
      </div>
      <SyntaxHighlighter
        style={isDark ? oneDark : oneLight}
        language={language}
        PreTag="div"
        className="!mt-0 !rounded-t-none"
        customStyle={{
          margin: 0,
          borderTopLeftRadius: 0,
          borderTopRightRadius: 0,
          fontSize: 12,
        }}
        {...props}
      >
        {code}
      </SyntaxHighlighter>
    </div>
  );
};
