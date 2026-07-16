import React from "react";

interface PageHeaderProps {
  title: string;
  subtitle?: string;
  icon?: React.ReactNode;
  children?: React.ReactNode;
  className?: string;
}

export const PageHeader: React.FC<PageHeaderProps> = ({
  title,
  subtitle,
  icon,
  children,
  className = "",
}) => {
  return (
    <header
      className={`py-2 px-3 sm:px-4 flex flex-wrap items-center gap-2 flex-shrink-0 ${className}`}
    >
      <div className="flex min-w-0 items-center space-x-3">
        {icon && <div>{icon}</div>}
        <div className="min-w-0">
          <h1 className="truncate text-lg sm:text-xl font-bold">{title}</h1>
          {subtitle && <p className="text-sm text-base-content/70 line-clamp-2">{subtitle}</p>}
        </div>
      </div>
      {children && (
        <div className="flex items-center gap-2 ml-auto flex-wrap justify-end">{children}</div>
      )}
    </header>
  );
};

export default PageHeader;
