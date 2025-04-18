import {useCallback, useEffect, useRef} from "react";
import {useGlobal} from "@/contexts/GlobalContext.jsx";

export const useMessage = (timeout = 5000) => {
    const { setError, setSuccess, setHint, setInfo } = useGlobal();
    const timeoutRef = useRef(null);

    const showMessage = useCallback((type, message) => {
        // Clear any existing timeout
        if (timeoutRef.current) {
            clearTimeout(timeoutRef.current);
        }

        // Show message
        switch (type) {
            case 'success':
                setSuccess(message);
                break;
            case 'error':
                setError(message);
                break;
            case 'hint':
                setHint(message);
                break;
            case 'info':
                setInfo(message);
                break;
        }

        // Set timeout to clear message
        timeoutRef.current = setTimeout(() => {
            switch (type) {
                case 'success':
                    setSuccess('');
                    break;
                case 'error':
                    setError('');
                    break;
                case 'hint':
                    setHint('');
                    break;
                case 'info':
                    setInfo('');
                    break;
            }
        }, timeout);
    }, [timeout, setError, setSuccess, setHint]);

    // Cleanup on unmount
    useEffect(() => {
        return () => {
            if (timeoutRef.current) {
                clearTimeout(timeoutRef.current);
            }
        };
    }, []);

    return showMessage;
}