import { useNavigate } from "react-router-dom";
import { useAuth } from "../../state/useAuth.ts";

/**
 * Ends the browser session and then opens the login page.
 *
 * The navigation must wait for {@link useAuth}'s `logout` to settle. Opening
 * `/login` while the session is still authenticated mounts a login form that
 * immediately redirects back into the app; once logout clears the session the
 * protected route sends the browser to `/login` again, and whatever was typed
 * into the first, discarded form is lost.
 */
export function useSignOut(): () => Promise<void> {
  const { logout } = useAuth();
  const navigate = useNavigate();

  return async () => {
    await logout();
    await navigate("/login", { replace: true });
  };
}
