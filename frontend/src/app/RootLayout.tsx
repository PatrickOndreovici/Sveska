import { Link, NavLink, Outlet, useNavigate } from "react-router";
import { ApiStatus } from "../components/ApiStatus";
import { LogoMark } from "../components/Logo";
import { ThemeToggle } from "../components/ThemeToggle";
import { Button } from "../components/ui/Button";
import { cx } from "../components/ui/cx";
import { navigationByRole, roleLabels } from "../features/auth/navigation";
import { useAuth } from "../features/auth/useAuth";
import { useHealth } from "../lib/api/useHealth";

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
      <header className="border-b border-rule">
        <div className="mx-auto flex min-h-16 w-full max-w-page flex-wrap items-center justify-between gap-x-6 gap-y-2 px-6 py-3">
          <div className="flex items-center gap-8">
            <Link to="/" className="inline-flex items-center gap-2.5 font-serif text-xl font-semibold tracking-tight text-ink">
              <LogoMark className="size-7" />
              Sveska
            </Link>
            {auth.user && (
              <nav aria-label="Main" className="flex gap-5 text-sm">
                {navigationByRole[auth.user.role].map((item) => (
                  <NavLink
                    key={item.to}
                    to={item.to}
                    end
                    className={({ isActive }) =>
                      cx("py-1", isActive ? "font-medium text-ink shadow-[inset_0_-2px_0_var(--color-accent)]" : "text-ink-muted hover:text-ink")
                    }
                  >
                    {item.label}
                  </NavLink>
                ))}
              </nav>
            )}
          </div>
          <div className="flex items-center gap-4">
            {auth.user && (
              <>
                <p className="flex items-center gap-2 text-sm whitespace-nowrap text-ink-muted">
                  <span className="hidden text-ink sm:inline">{auth.user.displayName}</span>
                  <span className="rounded-sm bg-paper-sunken px-1.5 py-0.5 text-xs font-medium text-ink-muted">
                    {roleLabels[auth.user.role]}
                  </span>
                </p>
                <Button variant="quiet" size="sm" onClick={() => void handleLogOut()}>
                  Log out
                </Button>
              </>
            )}
            <ThemeToggle />
          </div>
        </div>
      </header>

      <main className="mx-auto w-full max-w-page flex-1 px-6 py-16">
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
