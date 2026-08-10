import type { ReactNode } from "react";
import { useAuth } from "../../state/useAuth";

type RoleRouteProps = {
  children: ReactNode;
  requiredRole?: "admin";
  fallback?: ReactNode;
};

/** Enforces route metadata even when a user opens a privileged URL directly. */
export default function RoleRoute({ children, requiredRole, fallback = null }: RoleRouteProps): ReactNode {
  const { user } = useAuth();

  if (requiredRole && user?.role !== requiredRole) {
    return fallback;
  }
  return children;
}
