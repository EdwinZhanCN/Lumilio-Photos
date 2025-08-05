import { LayoutDashboard } from "lucide-react";

export default function UISettings() {
  return (
    <div>
      <h1 className="text-3xl font-bold my-3">Assets Page</h1>
      <div className="flex flex-row items-center gap-1">
        <LayoutDashboard size={20} />
        <h3 className="text-2xl font-bold my-2">Page Layout</h3>
      </div>
      <h3></h3>
    </div>
  );
}
