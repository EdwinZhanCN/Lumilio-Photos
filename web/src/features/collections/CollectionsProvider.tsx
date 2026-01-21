import React, { createContext, useReducer, ReactNode, useContext } from "react";
import { collectionsReducer, initialState } from "./collections.reducer";
import { CollectionsAction, CollectionsState } from "./types";

interface CollectionsContextType extends CollectionsState {
  dispatch: React.Dispatch<CollectionsAction>;
}

const CollectionsContext = createContext<CollectionsContextType | undefined>(
  undefined,
);

export const CollectionsProvider: React.FC<{ children: ReactNode }> = ({
  children,
}) => {
  const [state, dispatch] = useReducer(collectionsReducer, initialState);

  return (
    <CollectionsContext.Provider value={{ ...state, dispatch }}>
      {children}
    </CollectionsContext.Provider>
  );
};

export const useCollections = () => {
  const context = useContext(CollectionsContext);
  if (context === undefined) {
    throw new Error("useCollections must be used within a CollectionsProvider");
  }
  return context;
};
