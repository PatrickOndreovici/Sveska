import type { User } from "../lib/api/types";
import { cx } from "./ui/cx";

type Size = "sm" | "lg";

const sizeClasses: Record<Size, string> = {
  sm: "size-7 text-xs",
  lg: "size-32 text-4xl",
};

/** "AP" for Ana Petrović: the first letter of the first two words. */
export function initials(name: string): string {
  return name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((word) => Array.from(word)[0] ?? "")
    .join("")
    .toUpperCase();
}

/**
 * The user's profile picture, or their initials without one. It is
 * decorative by default, since the name is shown beside it; pass alt where
 * the picture itself is the subject.
 */
export function Avatar({ user, size = "sm", alt = "" }: { user: User; size?: Size; alt?: string }) {
  const classes = cx("shrink-0 rounded-full", sizeClasses[size]);
  if (user.avatarUrl) {
    return <img src={user.avatarUrl} alt={alt} className={cx(classes, "bg-paper-sunken object-cover")} />;
  }
  return (
    <span
      role={alt ? "img" : undefined}
      aria-label={alt || undefined}
      aria-hidden={alt ? undefined : true}
      className={cx(classes, "inline-flex items-center justify-center bg-accent-soft font-medium text-accent")}
    >
      {initials(user.displayName)}
    </span>
  );
}
