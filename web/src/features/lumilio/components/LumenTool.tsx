import { useState } from "react";
import { AgentEvent } from "@/services/agentService";
import GalleryView from "./LumenChat/GalleryView";

interface FilterToolProps {
  toolEvent: AgentEvent;
}

export function FilterTool({ toolEvent }: FilterToolProps) {
  const { action, output } = toolEvent;
  const [showGallery, setShowGallery] = useState(false);

  // Check if this is a filter tool
  const isFilterTool =
    action?.name?.toLowerCase().includes("filter") ||
    action?.name?.toLowerCase().includes("search");

  // Extract result count from output
  let resultCount = 0;

  if (output) {
    if (Array.isArray(output)) {
      resultCount = output.length;
    } else if (typeof output === "object" && "results" in output) {
      // Handle case where results might be nested in a results property
      const resultsArray = (output as any).results;
      if (Array.isArray(resultsArray)) {
        resultCount = resultsArray.length;
      }
    }
  }

  if (!isFilterTool) {
    return null;
  }

  return (
    <>
      <div className="my-3 rounded-lg border border-base-300 bg-base-200/30 p-3">
        {/* Tool Call Header */}
        <div className="mb-2 flex items-center gap-2 text-sm font-medium text-primary">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M22 3H2l8 9.46V19l4 2v-8.54L22 3z" />
          </svg>
          <span className="font-mono">{action?.name || "Filter"}</span>
          {resultCount > 0 && (
            <span className="ml-auto badge badge-primary badge-sm">
              {resultCount} items
            </span>
          )}
        </div>

        {/* Tool Result Summary */}
        <div className="text-sm">
          <p className="text-base-content/70 mb-2">
            Found {resultCount} items matching your filter criteria.
          </p>

          {/* Show Gallery Button */}
          {resultCount > 0 && (
            <button
              className="btn btn-sm btn-primary"
              onClick={() => setShowGallery(true)}
            >
              View in Gallery
            </button>
          )}
        </div>
      </div>

      {/* Gallery Modal/View */}
      {showGallery && (
        <div className="fixed inset-0 bg-base-100/95 z-50 flex flex-col">
          <div className="p-4 border-b border-base-300 flex justify-between items-center">
            <h2 className="text-xl font-bold">
              Filter Results: {action?.name || "Filter"}
            </h2>
            <button
              className="btn btn-sm btn-circle"
              onClick={() => setShowGallery(false)}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                className="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M6 18L18 6M6 6l12 12"
                />
              </svg>
            </button>
          </div>

          <GalleryView
            title={`${action?.name || "Filter"} Results`}
            count={resultCount}
            type="filter"
            onClear={() => setShowGallery(false)}
          />
        </div>
      )}
    </>
  );
}

// Simple fallback tool component for non-filter tools
export function SimpleTool({ toolEvent }: { toolEvent: AgentEvent }) {
  const { action, output } = toolEvent;

  return (
    <div className="my-3 rounded-lg border border-base-300 bg-base-200/30 p-3">
      {/* Tool Call Header */}
      {action && (
        <div className="mb-2 flex items-center gap-2 text-sm font-medium text-primary">
          <svg
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <path d="M18 3a3 3 0 0 0-3 3v12a3 3 0 0 0 3 3 3 3 0 0 0 3-3 3 3 0 0 0-3-3H6a3 3 0 0 0-3 3 3 3 0 0 0 3 3 3 3 0 0 0 3-3h12a3 3 0 0 0 3-3 3 3 0 0 0-3-3z" />
          </svg>
          <span className="font-mono">{action.name}</span>
        </div>
      )}

      {/* Tool Result */}
      {output && (
        <div className="text-sm">
          {typeof output === "string" ? (
            <div className="prose prose-sm max-w-none">
              <div className="whitespace-pre-wrap">{output}</div>
            </div>
          ) : (
            <div className="max-h-40 overflow-y-auto rounded bg-base-100 p-2">
              <pre className="text-xs">
                {(() => {
                  try {
                    return JSON.stringify(output, null, 2);
                  } catch {
                    return "Error displaying result";
                  }
                })()}
              </pre>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export default FilterTool;
