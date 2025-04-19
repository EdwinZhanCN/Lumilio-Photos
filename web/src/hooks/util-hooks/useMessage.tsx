import { useCallback, useEffect, useRef } from "react";
import { useGlobal } from "@/contexts/GlobalContext.tsx";

export const useMessage = (timeout = 5000) => {
    const { setError, setSuccess, setHint, setInfo } = useGlobal();

    // Specify number since in browsers setTimeout returns a numeric ID
    const timeoutRef = useRef<number | null>(null);

    const showMessage = useCallback(
        (type: "success" | "error" | "hint" | "info", message: string) => {
            // Clear any existing timeout
            if (timeoutRef.current !== null) {
                clearTimeout(timeoutRef.current);
            }

            // Show message
            switch (type) {
                case "success":
                    setSuccess(message);
                    break;
                case "error":
                    setError(message);
                    break;
                case "hint":
                    setHint(message);
                    break;
                case "info":
                    setInfo(message);
                    break;
            }

            // Set timeout to clear message
            timeoutRef.current = window.setTimeout(() => {
                switch (type) {
                    case "success":
                        setSuccess("");
                        break;
                    case "error":
                        setError("");
                        break;
                    case "hint":
                        setHint("");
                        break;
                    case "info":
                        setInfo("");
                        break;
                }
            }, timeout);
        },
        [timeout, setError, setSuccess, setHint, setInfo]
    );

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (timeoutRef.current !== null) {
                clearTimeout(timeoutRef.current);
            }
        };
    }, []);

    return showMessage;
};