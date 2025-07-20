import {
  InformationCircleIcon,
  ShareIcon,
  TrashIcon,
  ArrowDownTrayIcon,
} from "@heroicons/react/24/outline";

interface FullScreenToolbarProps {
  onToggleInfo: () => void;
}

const FullScreenToolbar = ({ onToggleInfo }: FullScreenToolbarProps) => {
  return (
    <div className="absolute top-0 left-0 right-0 bg-base-100/50 p-2 flex justify-between items-center z-10">
      <div>{/* Placeholder for future actions */}</div>
      <div className="flex items-center space-x-4">
        <button className="btn btn-ghost btn-sm" onClick={onToggleInfo}>
          <InformationCircleIcon className="h-6 w-6" />
        </button>
        <button className="btn btn-ghost btn-sm">
          <ShareIcon className="h-6 w-6" />
        </button>
        <button className="btn btn-ghost btn-sm">
          <ArrowDownTrayIcon className="h-6 w-6" />
        </button>
        <button className="btn btn-ghost btn-sm text-error">
          <TrashIcon className="h-6 w-6" />
        </button>
      </div>
    </div>
  );
};

export default FullScreenToolbar;
