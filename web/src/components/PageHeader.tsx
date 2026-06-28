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
      className={`py-2 px-4 flex items-center flex-shrink-0 ${className}`}
    >
      <div className="flex items-center space-x-3">
        {icon && <div>{icon}</div>}
        <div className="min-w-0">
          <h1 className="truncate text-xl font-bold">{title}</h1>
          {subtitle && (
            <p className="text-sm text-base-content/70 line-clamp-2">{subtitle}</p>
          )}
        </div>
      </div>
      {children && (
        <div className="flex items-center space-x-2 ml-auto">{children}</div>
      )}
    </header>
  );
};

export default PageHeader;
