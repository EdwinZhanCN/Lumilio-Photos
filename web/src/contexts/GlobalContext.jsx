import {createContext, useCallback, useContext, useEffect, useRef, useState} from 'react';

export const GlobalContext = createContext();


/**
 *
 * @author Edwin Zhan
 * @param {React.ReactNode} children
 * @constructor
 */
export default function GlobalProvider({children}){
    // Notification States
    const [error, setError] = useState('');
    const [success, setSuccess] = useState('');
    const [hint, setHint] = useState('');
    const [info, setInfo] = useState('');


    return (
        <GlobalContext.Provider value={{
            error,
            setError,
            success,
            setSuccess,
            hint,
            setHint,
            info,
            setInfo,
        }}>
            {children}
        </GlobalContext.Provider>
    )
}

export const useGlobal = () => useContext(GlobalContext);
