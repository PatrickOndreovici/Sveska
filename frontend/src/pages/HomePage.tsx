import { Eyebrow } from "../components/ui/Eyebrow";
import { useAuth } from "../features/auth/useAuth";
import { roleLabels } from "../features/auth/navigation";
import type { Role } from "../lib/api/types";

const introByRole: Record<Role, string> = {
  student:
    "This is your notebook. Your lessons, PDFs and vocabulary will collect here as those features arrive.",
  teacher: "Your classes, assignments and student work will collect here as those features arrive.",
};

/** The logged-in landing page. Rendered only inside RequireAuth. */
export function HomePage() {
  const { user } = useAuth();
  if (!user) return null;

  const firstName = user.displayName.split(/\s+/)[0] ?? user.displayName;

  return (
    <article className="max-w-prose">
      <Eyebrow>{roleLabels[user.role]} workspace</Eyebrow>
      <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Dobar dan, {firstName}.</h1>
      <p className="mt-6 font-serif text-lg leading-relaxed text-ink-muted">{introByRole[user.role]}</p>
      <hr className="mt-10 w-16 border-t-2 border-accent" />
      <p lang="sr" className="mt-6 font-serif italic text-ink-muted">
        Čitaj, piši, ponavljaj. · Читај, пиши, понављај.
      </p>
    </article>
  );
}
