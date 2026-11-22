import type { Role } from "../../lib/api/types";

export interface NavItem {
  to: string;
  label: string;
}

/**
 * Main navigation per role. Only routes the role may open are listed; each
 * phase adds its own entries (lessons and vocabulary for students, classes
 * and assignments for teachers).
 */
export const navigationByRole: Record<Role, readonly NavItem[]> = {
  student: [
    { to: "/", label: "My workspace" },
    { to: "/lessons", label: "Lessons" },
    { to: "/vocabulary", label: "Vocabulary" },
    { to: "/review", label: "Review" },
  ],
  teacher: [{ to: "/", label: "Teaching" }],
};

export const roleLabels: Record<Role, string> = {
  student: "Student",
  teacher: "Teacher",
};
