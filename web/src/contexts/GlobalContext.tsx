import {
  createContext,
  useContext,
  useState,
  ReactNode,
  Dispatch,
  SetStateAction,
} from "react";

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
  const [online, setOnline] = useState(false);

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
