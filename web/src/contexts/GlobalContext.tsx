import {
  createContext,
  useContext,
  useState,
  useCallback,
  ReactNode,
  Dispatch,
  SetStateAction,
} from "react";

export type GlobalMessageType = "success" | "error" | "hint" | "info";

export interface GlobalNotification {
  id: string;
  type: GlobalMessageType;
  message: string;
  createdAt: number;
  read: boolean;
}

// 1. Define the shape of your GlobalContext
interface GlobalContextType {
  error: string;
  setError: Dispatch<SetStateAction<string>>;
  success: string;
  setSuccess: Dispatch<SetStateAction<string>>;
  hint: string;
  setHint: Dispatch<SetStateAction<string>>;
  info: string;
  setInfo: Dispatch<SetStateAction<string>>;
  notifications: GlobalNotification[];
  addNotification: (
    type: GlobalMessageType,
    message: string,
    options?: { duration?: number },
  ) => string | null;
  removeNotification: (id: string) => void;
  markNotificationRead: (id: string) => void;
  markAllNotificationsRead: () => void;
  clearNotifications: () => void;
  online: boolean;
  setOnline: Dispatch<SetStateAction<boolean>>;
}

// 2. Provide a default context (to avoid undefined issues)
const defaultContext: GlobalContextType = {
  error: "",
  setError: () => {},
  success: "",
  setSuccess: () => {},
  hint: "",
  setHint: () => {},
  info: "",
  setInfo: () => {},
  notifications: [],
  addNotification: () => null,
  removeNotification: () => {},
  markNotificationRead: () => {},
  markAllNotificationsRead: () => {},
  clearNotifications: () => {},
  online: false,
  setOnline: () => {},
};

// 3. Create the context
export const GlobalContext = createContext<GlobalContextType>(defaultContext);

// 4. Define a type for your provider's props
interface GlobalProviderProps {
  children: ReactNode;
}

// 5. Create the provider with typed state
export default function GlobalProvider({ children }: GlobalProviderProps) {
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [hint, setHint] = useState("");
  const [info, setInfo] = useState("");
  const [notifications, setNotifications] = useState<GlobalNotification[]>([]);
  const [online, setOnline] = useState(false);

  const removeNotification = useCallback((id: string) => {
    setNotifications((prev) => prev.filter((item) => item.id !== id));
  }, []);

  const addNotification = useCallback(
    (
      type: GlobalMessageType,
      message: string,
      options?: { duration?: number },
    ) => {
      const trimmed = message.trim();
      if (!trimmed) return null;

      const id =
        typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
          ? crypto.randomUUID()
          : `${Date.now()}-${Math.random().toString(36).slice(2)}`;

      setNotifications((prev) => {
        const next: GlobalNotification = {
          id,
          type,
          message: trimmed,
          createdAt: Date.now(),
          read: false,
        };
        return [next, ...prev].slice(0, 200);
      });

      void options;

      return id;
    },
    [],
  );

  const markNotificationRead = useCallback((id: string) => {
    setNotifications((prev) =>
      prev.map((item) => (item.id === id ? { ...item, read: true } : item)),
    );
  }, []);

  const markAllNotificationsRead = useCallback(() => {
    setNotifications((prev) => prev.map((item) => ({ ...item, read: true })));
  }, []);

  const clearNotifications = useCallback(() => {
    setNotifications([]);
  }, []);

  return (
    <GlobalContext.Provider
      value={{
        error,
        setError,
        success,
        setSuccess,
        hint,
        setHint,
        info,
        setInfo,
        notifications,
        addNotification,
        removeNotification,
        markNotificationRead,
        markAllNotificationsRead,
        clearNotifications,
        online,
        setOnline,
      }}
    >
      {children}
    </GlobalContext.Provider>
  );
}

// 6. Create a typed hook to consume the context
export function useGlobal() {
  return useContext(GlobalContext);
}
