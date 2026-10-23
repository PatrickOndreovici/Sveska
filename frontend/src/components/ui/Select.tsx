import type { ComponentProps } from "react";
import { cx } from "./cx";

type Size = "sm" | "md";

const sizeClasses: Record<Size, string> = {
  sm: "h-8 pl-3 pr-8 text-sm",
  md: "h-10 pl-3 pr-9",
};

export type SelectProps = Omit<ComponentProps<"select">, "size"> & {
  size?: Size;
};

/**
 * Native select with the browser's arrow replaced by our own chevron, so the
 * padding and height match Button and TextField in every browser.
 */
export function Select({ size = "md", className, ...rest }: SelectProps) {
  return (
    <div className={cx("relative", className)}>
      <select
        className={cx(
          "w-full appearance-none rounded-sm border border-rule-strong bg-paper-raised text-ink hover:border-ink-muted",
          sizeClasses[size],
        )}
        {...rest}
      />
      <svg
        aria-hidden="true"
        viewBox="0 0 16 16"
        className="pointer-events-none absolute top-1/2 right-2.5 size-4 -translate-y-1/2 text-ink-muted"
      >
        <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>
    </div>
  );
}
