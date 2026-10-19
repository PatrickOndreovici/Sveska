import { Navigate, Outlet, useLocation } from "react-router";
import { useAuth } from "./useAuth";

export interface RedirectState {
  from?: string;
}

function CheckingSession() {
  return (
    <p role="status" className="text-sm text-ink-muted">
      Checking your session…
    </p>
  );
}

/** Renders child routes only for logged-in users; others go to /login and come back afterwards. */
export function RequireAuth() {
  const auth = useAuth();
  const location = useLocation();

  if (auth.status === "loading") return <CheckingSession />;
  if (auth.status === "anonymous") {
    const state: RedirectState = { from: location.pathname + location.search };
    return <Navigate to="/login" replace state={state} />;
  }
  return <Outlet />;
}

/** Renders child routes only for logged-out users, such as the login and sign-up pages. */
export function GuestOnly() {
  const auth = useAuth();
  if (auth.status === "loading") return <CheckingSession />;
  if (auth.status === "authenticated") return <Navigate to="/" replace />;
  return <Outlet />;
}
