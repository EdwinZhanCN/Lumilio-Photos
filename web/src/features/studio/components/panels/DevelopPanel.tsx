import { AdjustmentsHorizontalIcon } from "@heroicons/react/24/outline";

export function DevelopPanel() {
  return (
    <div className="h-full flex items-center justify-center text-center p-4 rounded-lg bg-base-100 min-h-[400px]">
      <div>
        <AdjustmentsHorizontalIcon className="w-12 h-12 mx-auto text-base-content/50" />
        <h3 className="mt-2 text-lg font-semibold">Development Tools</h3>
        <p className="mt-1 text-sm text-base-content/70">
          Color adjustments, cropping, and other tools will be available here
          soon.
        </p>
      </div>
    </div>
  );
}
