import { Link, Outlet } from "react-router";
import { ApiStatus } from "../components/ApiStatus";
import { LogoMark } from "../components/Logo";
import { ThemeToggle } from "../components/ThemeToggle";
import { useHealth } from "../lib/api/useHealth";

export function RootLayout() {
  const health = useHealth();

  return (
    <div className="flex min-h-dvh flex-col">
      <header className="border-b border-rule">
        <div className="mx-auto flex h-16 w-full max-w-page items-center justify-between gap-4 px-6">
          <Link to="/" className="inline-flex items-center gap-2.5 font-serif text-xl font-semibold tracking-tight text-ink">
            <LogoMark className="size-7" />
            Sveska
          </Link>
          <ThemeToggle />
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
