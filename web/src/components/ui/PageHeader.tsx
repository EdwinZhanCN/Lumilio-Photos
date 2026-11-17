import React from "react";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
  /** Optional aligned content frame for pages with a constrained main column. */
  contentClassName?: string;
  /** Optional title scale override for a page-specific display heading. */
  titleClassName?: string;
}

export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  subtitle,
  icon,
  children,
  className = "",
  contentClassName,
  titleClassName = "",
}) => {
  const content = (
    <>
      <div className="flex min-w-0 items-center space-x-3">
        {icon && <div>{icon}</div>}
        <div className="min-w-0">
          <h1 className={`truncate text-lg font-bold sm:text-xl ${titleClassName}`}>{title}</h1>
          {subtitle && <p className="line-clamp-2 text-sm text-base-content/70">{subtitle}</p>}
        </div>
      </div>
      {children && (
        <div className="ml-auto flex flex-wrap items-center justify-end gap-2">{children}</div>
      )}
    </>
  );

  return (
    <header
      className={
        contentClassName
          ? `flex flex-shrink-0 ${className}`
          : `flex flex-shrink-0 flex-wrap items-center gap-2 px-3 py-2 sm:px-4 ${className}`
      }
    >
      {contentClassName ? (
        <div className={`flex w-full flex-wrap items-center gap-2 ${contentClassName}`}>
          {content}
        </div>
      ) : (
        content
      )}
    </header>
  );
};

export default PageHeader;
