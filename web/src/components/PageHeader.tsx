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
      className={`py-2 px-4 flex justify-between items-center flex-shrink-0 ${className}`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-3">
          {icon && <div>{icon}</div>}
          <div>
            <h1 className="text-xl font-bold">{title}</h1>
            {subtitle && <p className="text-sm">{subtitle}</p>}
          </div>
        </div>
        {children && (
          <div className="flex items-center space-x-2">{children}</div>
        )}
      </div>
    </header>
  );
};

export default PageHeader;
