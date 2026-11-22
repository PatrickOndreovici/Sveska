import { useEffect, useRef, useState } from "react";
import { Link, NavLink, useLocation } from "react-router";
import { Avatar } from "../components/Avatar";
import { LogoMark } from "../components/Logo";
import { ThemeToggle } from "../components/ThemeToggle";
import { Button } from "../components/ui/Button";
import { cx } from "../components/ui/cx";
import { navigationByRole, roleLabels } from "../features/auth/navigation";
import type { User } from "../lib/api/types";

type SiteHeaderProps = {
  user: User | null;
  onLogOut: () => void;
};

/**
 * Wide screens show everything in one row. Narrow screens show the logo and
 * a menu button; the menu holds navigation, account and theme controls.
 */
export function SiteHeader({ user, onLogOut }: SiteHeaderProps) {
  const location = useLocation();
  // The menu remembers the path it was opened on, so navigating closes it
  // without an effect that resets state.
  const [openAt, setOpenAt] = useState<string | null>(null);
  const menuOpen = openAt === location.pathname;
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!menuOpen) return;
    function closeOnEscape(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      setOpenAt(null);
      menuButtonRef.current?.focus();
    }
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [menuOpen]);

  return (
    <header className="border-b border-rule">
      <div className="mx-auto flex h-16 w-full max-w-page items-center justify-between gap-6 px-6">
        <div className="flex items-center gap-8">
          <Link to="/" className="inline-flex items-center gap-2.5 font-serif text-xl font-semibold tracking-tight text-ink">
            <LogoMark className="size-7" />
            Sveska
          </Link>
          {user && (
            <nav aria-label="Main" className="hidden gap-5 text-sm md:flex">
              <NavItems user={user} />
            </nav>
          )}
        </div>

        {user ? (
          <>
            <div className="hidden items-center gap-4 md:flex">
              <AccountSummary user={user} />
              <SettingsLink />
              <Button variant="quiet" size="sm" onClick={onLogOut}>
                Log out
              </Button>
              <ThemeToggle />
            </div>
            <button
              ref={menuButtonRef}
              type="button"
              aria-expanded={menuOpen}
              aria-controls="site-menu"
              onClick={() => {
                setOpenAt(menuOpen ? null : location.pathname);
              }}
              className="-mr-2 inline-flex size-10 items-center justify-center rounded-sm text-ink hover:bg-paper-sunken md:hidden"
            >
              <span className="sr-only">Menu</span>
              <MenuIcon open={menuOpen} />
            </button>
          </>
        ) : (
          <ThemeToggle />
        )}
      </div>

      {user && menuOpen && (
        <div id="site-menu" className="border-t border-rule bg-paper-raised md:hidden">
          <div className="mx-auto grid w-full max-w-page gap-5 px-6 py-5">
            <nav aria-label="Main" className="grid gap-1">
              <NavItems
                user={user}
                large
                onNavigate={() => {
                  setOpenAt(null);
                }}
              />
            </nav>
            <hr className="border-rule" />
            <div className="flex items-center justify-between gap-4">
              <AccountSummary user={user} />
              <SettingsLink
                onNavigate={() => {
                  setOpenAt(null);
                }}
              />
            </div>
            <div className="flex items-center justify-between gap-4">
              <ThemeToggle />
              <Button variant="secondary" size="sm" onClick={onLogOut}>
                Log out
              </Button>
            </div>
          </div>
        </div>
      )}
    </header>
  );
}

function NavItems({ user, large = false, onNavigate }: { user: User; large?: boolean; onNavigate?: () => void }) {
  return navigationByRole[user.role].map((item) => (
    <NavLink
      key={item.to}
      to={item.to}
      // Only the workspace needs an exact match; "Lessons" stays current on a lesson's own page.
      end={item.to === "/"}
      onClick={onNavigate}
      className={({ isActive }) =>
        cx(
          large ? "rounded-sm px-3 py-2.5 text-base" : "py-1",
          isActive
            ? large
              ? "bg-accent-soft font-medium text-ink"
              : "font-medium text-ink shadow-[inset_0_-2px_0_var(--color-accent)]"
            : "text-ink-muted hover:text-ink",
        )
      }
    >
      {item.label}
    </NavLink>
  ));
}

function AccountSummary({ user }: { user: User }) {
  return (
    <p className="flex min-w-0 items-center gap-2 text-sm text-ink-muted">
      <Avatar user={user} />
      <span className="truncate text-ink">{user.displayName}</span>
      <span className="shrink-0 rounded-sm bg-paper-sunken px-1.5 py-0.5 text-xs font-medium text-ink-muted">
        {roleLabels[user.role]}
      </span>
    </p>
  );
}

function SettingsLink({ onNavigate }: { onNavigate?: () => void }) {
  return (
    <NavLink
      to="/settings"
      onClick={onNavigate}
      className={({ isActive }) =>
        cx(
          "shrink-0 text-sm underline-offset-4 hover:text-ink hover:underline",
          isActive ? "font-medium text-ink" : "text-ink-muted",
        )
      }
    >
      Settings
    </NavLink>
  );
}

function MenuIcon({ open }: { open: boolean }) {
  return (
    <svg viewBox="0 0 24 24" className="size-6" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" aria-hidden="true">
      {open ? <path d="M6 6l12 12M18 6L6 18" /> : <path d="M4 7h16M4 12h16M4 17h16" />}
    </svg>
  );
}
