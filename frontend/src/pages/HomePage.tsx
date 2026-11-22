import { Eyebrow } from "../components/ui/Eyebrow";
import { useAuth } from "../features/auth/useAuth";
import { roleLabels } from "../features/auth/navigation";
import { StudentDashboard } from "../features/dashboard/StudentDashboard";

/** The logged-in landing page: a student's dashboard, or a teacher's welcome. Rendered only inside RequireAuth. */
export function HomePage() {
  const { user } = useAuth();
  if (!user) return null;

  const firstName = user.displayName.split(/\s+/)[0] ?? user.displayName;

  return (
    <article>
      <Eyebrow>{roleLabels[user.role]} workspace</Eyebrow>
      <h1 className="mt-3 font-serif text-4xl text-ink sm:text-display">Dobar dan, {firstName}.</h1>
      {user.role === "student" ? (
        <StudentDashboard />
      ) : (
        <div className="max-w-prose">
          <p className="mt-6 font-serif text-lg leading-relaxed text-ink-muted">
            Your classes, assignments and student work will collect here as those features arrive.
          </p>
          <hr className="mt-10 w-16 border-t-2 border-accent" />
          <p lang="sr" className="mt-6 font-serif italic text-ink-muted">
            Čitaj, piši, ponavljaj. · Читај, пиши, понављај.
          </p>
        </div>
      )}
    </article>
  );
}
