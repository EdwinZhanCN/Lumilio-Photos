import React from "react";

type LinkBlockProps = {
  href?: string;
  title?: string;
  className?: string;
  children?: React.ReactNode;
  [key: string]: any;
};

export const Link: React.FC<LinkBlockProps> = ({
  href,
  title,
  className = "",
  children,
  ...props
}) => {
  const isExternal = href && (href.startsWith("http") || href.startsWith("https"));
  const isEmail = href && href.startsWith("mailto:");
  const isPhone = href && href.startsWith("tel:");

  const getFaviconUrl = () => {
    if (isExternal && href) {
      try {
        const url = new URL(href);
        return `https://www.google.com/s2/favicons?domain=${url.hostname}&sz=32`;
      } catch {
        return null;
      }
    }
    return null;
  };

  const truncateText = (text: string) => {
    if (text.length > 8) {
      return text.substring(0, 8) + "...";
    }
    return text;
  };

  const getFullText = () => {
    if (typeof children === "string") {
      return children;
    }
    return href || "";
  };

  const getLinkIcon = () => {
    const faviconUrl = getFaviconUrl();

    if (faviconUrl) {
      return (
        <img
          src={faviconUrl}
          alt=""
          className="w-3 h-3 ml-1.5 inline-block"
          onError={(e) => {
            e.currentTarget.style.display = "none";
            e.currentTarget.nextElementSibling?.classList.remove("hidden");
          }}
        />
      );
    }

    if (isExternal) {
      return (
        <svg
          className="w-3 h-3 ml-1.5 inline-block"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
          />
        </svg>
      );
    }
    if (isEmail) {
      return (
        <svg
          className="w-3 h-3 ml-1.5 inline-block"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M3 8l7.89 4.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
          />
        </svg>
      );
    }
    if (isPhone) {
      return (
        <svg
          className="w-3 h-3 ml-1.5 inline-block"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
          />
        </svg>
      );
    }
    return null;
  };

  const capsuleStyles =
    "inline-flex items-center bg-base-200 hover:bg-base-300 text-base-content/60 px-1 py-1 m-0.5 rounded-full text-xs font-medium transition-all duration-200 no-underline";

  const displayText = typeof children === "string" ? truncateText(children) : children;

  const fullText = getFullText();

  if (!href) {
    return (
      <span
        className={`inline-flex items-center bg-base-200 text-base-content/50 px-2 py-1 rounded-full text-xs font-medium ${className}`}
        title={fullText}
        {...props}
      >
        {displayText}
      </span>
    );
  }

  return (
    <a
      href={href}
      title={title || fullText}
      className={`${capsuleStyles} ${className}`}
      target={isExternal ? "_blank" : undefined}
      rel={isExternal ? "noopener noreferrer" : undefined}
      {...props}
    >
      <span>{displayText}</span>
      {getLinkIcon()}
    </a>
  );
};
