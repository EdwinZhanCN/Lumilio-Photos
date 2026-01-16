import React from "react";
import {
  getFormatGroups,
  getSupportedFormatsSummary,
  type FormatGroup,
} from "@/lib/utils/accept-file-extensions";
import {
  PhotoIcon,
  VideoCameraIcon,
  MusicalNoteIcon,
  CameraIcon,
} from "@heroicons/react/24/outline";

interface SupportedFormatsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

const SupportedFormatsModal: React.FC<SupportedFormatsModalProps> = ({
  isOpen,
  onClose,
}) => {
  const formatGroups = getFormatGroups();
  const summary = getSupportedFormatsSummary();

  const getCategoryIcon = (category: string) => {
    switch (category) {
      case "Photos":
        return <PhotoIcon className="w-5 h-5" />;
      case "RAW Formats":
        return <CameraIcon className="w-5 h-5" />;
      case "Videos":
        return <VideoCameraIcon className="w-5 h-5" />;
      case "Audio":
        return <MusicalNoteIcon className="w-5 h-5" />;
      default:
        return null;
    }
  };

  const getCategoryColor = (category: string) => {
    switch (category) {
      case "Photos":
        return "badge-success";
      case "RAW Formats":
        return "badge-warning";
      case "Videos":
        return "badge-info";
      case "Audio":
        return "badge-secondary";
      default:
        return "badge-neutral";
    }
  };

  if (!isOpen) return null;

  return (
    <>
      {/* Modal backdrop */}
      <div
        className="fixed inset-0 bg-black/50 z-40 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal dialog */}
      <dialog open className="modal modal-open z-50">
        <div className="modal-box max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
          {/* Header */}
          <div className="flex items-center justify-between mb-4">
            <h3 className="font-bold text-2xl">Supported File Formats</h3>
            <button
              onClick={onClose}
              className="btn btn-sm btn-circle btn-ghost"
              aria-label="Close"
            >
              ✕
            </button>
          </div>

          {/* Summary */}
          <div className="alert alert-info mb-6">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              className="stroke-current shrink-0 w-6 h-6"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span className="text-sm">{summary}</span>
          </div>

          {/* Format groups */}
          <div className="overflow-y-auto flex-1 pr-2 custom-scrollbar">
            <div className="space-y-6">
              {formatGroups.map((group: FormatGroup) => (
                <div key={group.category} className="card bg-base-200">
                  <div className="card-body p-4">
                    {/* Category header */}
                    <div className="flex items-center gap-3 mb-3">
                      <div className="text-primary">
                        {getCategoryIcon(group.category)}
                      </div>
                      <h4 className="font-semibold text-lg">
                        {group.category}
                      </h4>
                      <div
                        className={`badge ${getCategoryColor(group.category)} badge-sm`}
                      >
                        {group.formats.length} formats
                      </div>
                    </div>

                    {/* Format grid */}
                    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2">
                      {group.formats.map((format) => (
                        <div
                          key={format.ext}
                          className="flex items-center gap-2 p-2 bg-base-100 rounded hover:bg-base-300 transition-colors"
                        >
                          <code className="text-xs font-mono text-primary">
                            {format.ext}
                          </code>
                          <span className="text-xs text-base-content/70 truncate">
                            {format.name}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Footer */}
          <div className="modal-action mt-4">
            <button onClick={onClose} className="btn btn-primary">
              Got it!
            </button>
          </div>
        </div>

        {/* Custom scrollbar styles */}
        <style>{`
          .custom-scrollbar::-webkit-scrollbar {
            width: 6px;
          }
          .custom-scrollbar::-webkit-scrollbar-track {
            background: transparent;
          }
          .custom-scrollbar::-webkit-scrollbar-thumb {
            background: hsl(var(--bc) / 0.2);
            border-radius: 3px;
          }
          .custom-scrollbar::-webkit-scrollbar-thumb:hover {
            background: hsl(var(--bc) / 0.3);
          }
        `}</style>
      </dialog>
    </>
  );
};

export default SupportedFormatsModal;
