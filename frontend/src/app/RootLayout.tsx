import { Outlet, useNavigate } from "react-router";
import { ApiStatus } from "../components/ApiStatus";
import { useAuth } from "../features/auth/useAuth";
import { useHealth } from "../lib/api/useHealth";
import { SiteHeader } from "./SiteHeader";

export function RootLayout() {
  const health = useHealth();
  const auth = useAuth();
  const navigate = useNavigate();

  async function handleLogOut() {
    try {
      await auth.logOut();
      await navigate("/login");
    } catch (error) {
      console.error("log out failed", error);
    }
  }

  return (
    <div className="flex min-h-dvh flex-col">
      <SiteHeader user={auth.user} onLogOut={() => void handleLogOut()} />

      <main className="mx-auto w-full max-w-page flex-1 px-6 py-10 sm:py-16">
        <Outlet />
      </main>

      <footer className="border-t border-rule">
        <div className="mx-auto flex h-14 w-full max-w-page items-center justify-between px-6">
          <ApiStatus state={health} />
          <span className="text-sm text-ink-subtle">Development build</span>
        </div>
      </footer>
    </div>
  );
}
