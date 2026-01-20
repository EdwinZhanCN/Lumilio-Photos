import { Brain } from "lucide-react";
import React, { useState } from "react";

interface ThinkBlockProps {
  open?: boolean;
  className?: string;
  children?: React.ReactNode;
  [key: string]: any;
}

export const ThinkBlock: React.FC<ThinkBlockProps> = ({
  open = false,
  className = "",
  children,
  ...props
}) => {
  const [isOpen, setIsOpen] = useState(open);

  const toggleOpen = () => {
    setIsOpen(!isOpen);
  };

  // Extract summary from children if it exists
  const childrenArray = React.Children.toArray(children);
  const summaryElement = childrenArray.find(
    (child) => React.isValidElement(child) && child.type === "summary",
  );

  const contentElements = childrenArray.filter(
    (child) => !React.isValidElement(child) || child.type !== "summary",
  );

  const summaryText = summaryElement
    ? React.isValidElement(summaryElement) && summaryElement.props
      ? (summaryElement.props as any).children
      : "Details"
    : "Think about this...";

  return (
    <div
      className={`border border-base-300 rounded-lg my-4 overflow-hidden ${className}`}
      {...props}
    >
      <button
        onClick={toggleOpen}
        className="w-full px-2 py-3 text-left bg-base-200 hover:bg-base-300 transition-colors duration-200 flex items-center gap-5 cursor-pointer"
      >
        <span className="font-medium text-base-content flex items-center">
          <Brain className="mx-1.5 text-primary" strokeWidth={1.25} />
          {summaryText}
        </span>
        <svg
          className={`w-5 h-5 text-base-content/60 transition-transform duration-200 ${
            isOpen ? "transform rotate-180" : ""
          }`}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>

      <div
        className={`transition-all duration-300 ease-in-out overflow-hidden ${
          isOpen ? "max-h-none opacity-100" : "max-h-0 opacity-0"
        }`}
      >
        <div className="px-4 py-3 bg-base-100 border-t border-base-300">
          <div className="prose max-w-none text-base-content/80">
            {contentElements}
          </div>
        </div>
      </div>
    </div>
  );
};
