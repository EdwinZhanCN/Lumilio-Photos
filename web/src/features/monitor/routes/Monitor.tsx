import { Activity } from "lucide-react";
import PageHeader from "@/components/PageHeader";
import { StatMonitor, TaskMonitor, QueueList } from "../components";

export default function Monitor() {
  return (
    <div className="flex flex-col h-full">
      {/* PageHeader - Fixed at top */}
      <PageHeader
        title="Queue Monitor"
        subtitle="Real-time job queue monitoring"
        icon={<Activity className="w-6 h-6 text-primary" />}
      />

      {/* Content Area - Flex container */}
      <div className="flex-1 flex flex-col min-h-0 container mx-auto p-4 space-y-4">
        {/* Stats Section */}
        <StatMonitor />

        {/* Two Column Layout for Queues and Jobs */}
        <div className="flex-1 grid grid-cols-1 lg:grid-cols-2 gap-4 min-h-0">
          {/* Queue List Panel */}
          <div className="flex flex-col min-h-0">
            <h2 className="text-lg font-semibold mb-3 px-1">Active Queues</h2>
            <div className="flex-1 min-h-0">
              <QueueList />
            </div>
          </div>

          {/* Job List Panel */}
          <div className="flex flex-col min-h-0">
            <h2 className="text-lg font-semibold mb-3 px-1">Recent Jobs</h2>
            <div className="flex-1 min-h-0">
              <TaskMonitor />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
