import type { Role } from "../../lib/api/types";
import { cx } from "../../components/ui/cx";

const options: { value: Role; title: string; description: string }[] = [
  {
    value: "student",
    title: "I'm learning Serbian",
    description: "Lessons, PDFs, vocabulary and quizzes. You can invite a teacher later.",
  },
  {
    value: "teacher",
    title: "I teach Serbian",
    description: "Classes, assignments, grading and feedback for your students.",
  },
];

/** Role picker for sign-up. The choice is permanent, so both options say plainly what they include. */
export function RoleChoice({ error }: { error?: string | undefined }) {
  const errorId = error ? "signup-role-error" : undefined;
  return (
    <fieldset className="grid gap-2" aria-describedby={errorId}>
      <legend className="mb-1.5 text-sm font-medium text-ink">How will you use Sveska?</legend>
      {options.map((option, index) => (
        <label
          key={option.value}
          className={cx(
            "flex cursor-pointer gap-3 rounded-sm border bg-paper-raised p-3 has-[:checked]:border-accent has-[:checked]:bg-accent-soft",
            error ? "border-danger" : "border-rule-strong hover:border-ink-muted",
          )}
        >
          <input
            type="radio"
            name="role"
            value={option.value}
            aria-invalid={error && index === 0 ? true : undefined}
            className="mt-1 accent-accent"
          />
          <span className="grid gap-0.5">
            <span className="font-medium text-ink">{option.title}</span>
            <span className="text-sm text-ink-muted">{option.description}</span>
          </span>
        </label>
      ))}
      {error && (
        <p id={errorId} className="text-sm text-danger">
          {error}
        </p>
      )}
    </fieldset>
  );
}

export function isRole(value: string): value is Role {
  return value === "student" || value === "teacher";
}
