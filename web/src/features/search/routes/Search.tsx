import PageHeader from "@/components/PageHeader";
import { MagnifyingGlassIcon } from "@heroicons/react/24/outline";

export default function Search() {
  return (
    <div>
      <PageHeader
        title="Search"
        icon={<MagnifyingGlassIcon className="w-6 h-6 text-primary" />}
      />
    </div>
  );
}
