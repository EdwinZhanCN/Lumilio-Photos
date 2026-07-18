import React, { createContext, useReducer, ReactNode, useContext } from "react";
import { albumsReducer, initialState } from "./reducer";
import type { AlbumsAction, AlbumsState } from "./types.ts";

interface AlbumsContextType extends AlbumsState {
  dispatch: React.Dispatch<AlbumsAction>;
}

const AlbumsContext = createContext<AlbumsContextType | undefined>(undefined);

export const AlbumsProvider: React.FC<{ children: ReactNode }> = ({ children }) => {
  const [state, dispatch] = useReducer(albumsReducer, initialState);

  return (
    <AlbumsContext.Provider value={{ ...state, dispatch }}>
      {children}
    </AlbumsContext.Provider>
  );
};

export const useAlbumsState = () => {
  const context = useContext(AlbumsContext);
  if (context === undefined) {
    throw new Error("useAlbumsState must be used within an AlbumsProvider");
  }
  return context;
};
