import { useContext } from "react";
import { AssetsNavigationContext } from "../AssetsProvider";

export const useAssetsNavigation = () => {
  const context = useContext(AssetsNavigationContext);
  if (context === undefined) {
    throw new Error(
      "useAssetsNavigation must be used within an AssetsProvider",
    );
  }
  return context;
};
