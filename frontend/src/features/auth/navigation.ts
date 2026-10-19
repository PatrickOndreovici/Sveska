import type { Role } from "../../lib/api/types";

export interface NavItem {
  to: string;
  label: string;
}

/**
 * Main navigation per role. Only routes that exist are listed; each phase
 * adds its own entries (lessons and vocabulary for students, classes and
 * assignments for teachers).
 */
export const navigationByRole: Record<Role, readonly NavItem[]> = {
  student: [
    { to: "/", label: "My workspace" },
    { to: "/vocabulary", label: "Vocabulary" },
  ],
  teacher: [{ to: "/", label: "Teaching" }],
};

export const roleLabels: Record<Role, string> = {
  student: "Student",
  teacher: "Teacher",
};
