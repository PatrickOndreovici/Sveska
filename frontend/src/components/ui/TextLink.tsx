import { Link, type LinkProps } from "react-router";
import { cx } from "./cx";

export function TextLink({ className, ...rest }: LinkProps) {
  return (
    <Link
      className={cx(
        "text-accent underline decoration-rule-strong underline-offset-4 hover:decoration-accent",
        className,
      )}
      {...rest}
    />
  );
}
