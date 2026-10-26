import type { ComponentProps } from "react";
import { cx } from "./cx";

type Variant = "primary" | "secondary" | "quiet" | "danger";
type Size = "sm" | "md";

const variantClasses: Record<Variant, string> = {
  primary: "bg-accent text-on-accent hover:bg-accent-hover",
  secondary: "border border-rule-strong bg-paper-raised text-ink hover:border-ink",
  quiet: "text-ink-muted hover:text-ink underline-offset-4 hover:underline",
  // For the confirming button of a destructive action only.
  danger: "bg-danger text-on-accent hover:opacity-90",
};

const sizeClasses: Record<Size, string> = {
  sm: "h-8 px-3 text-sm",
  md: "h-10 px-4 text-sm",
};

export type ButtonProps = ComponentProps<"button"> & {
  variant?: Variant;
  size?: Size;
};

/** The button's look, also for a link that should read as a button. */
export function buttonClasses(variant: Variant = "primary", size: Size = "md", className?: string): string {
  return cx(
    "inline-flex items-center justify-center gap-2 rounded-sm font-medium whitespace-nowrap transition-colors",
    "disabled:cursor-not-allowed disabled:opacity-50",
    variantClasses[variant],
    sizeClasses[size],
    className,
  );
}

export function Button({ variant = "primary", size = "md", className, type = "button", ...rest }: ButtonProps) {
  return <button type={type} className={buttonClasses(variant, size, className)} {...rest} />;
}
