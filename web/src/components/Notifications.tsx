import { Toaster } from "@/components/ui/Sonner";

function Notifications() {
    return (
        <Toaster position="top-right" visibleToasts={5} />
    );
}

export default Notifications;
