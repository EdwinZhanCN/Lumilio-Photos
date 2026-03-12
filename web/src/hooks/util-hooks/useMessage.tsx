import { useCallback } from "react";
import { useGlobal } from "@/contexts/GlobalContext.tsx";
import { toast } from "sonner";

export const useMessage = (timeout = 5000) => {
    const { addNotification, markNotificationRead } = useGlobal();

    const showMessage = useCallback(
        (type: "success" | "error" | "hint" | "info", message: string) => {
            const id = addNotification(type, message, { duration: timeout });
            const commonOptions = {
                duration: timeout,
                onDismiss: () => {
                    if (id) {
                        markNotificationRead(id);
                    }
                },
            };

            switch (type) {
                case "success":
                    toast.success(message, commonOptions);
                    break;
                case "error":
                    toast.error(message, commonOptions);
                    break;
                case "info":
                    toast.info(message, commonOptions);
                    break;
                case "hint":
                default:
                    toast(message, commonOptions);
                    break;
            }
        },
        [addNotification, markNotificationRead, timeout]
    );

    return showMessage;
};
