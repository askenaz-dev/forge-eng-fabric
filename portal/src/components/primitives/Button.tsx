import { ButtonHTMLAttributes, forwardRef, ReactNode } from "react";
import { cx } from "./cx";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "default" | "xs";

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant;
  size?: ButtonSize;
  leading?: ReactNode;
  trailing?: ReactNode;
};

const VARIANT_CLASS: Record<ButtonVariant, string> = {
  primary:   "btn--primary",
  secondary: "btn--secondary",
  ghost:     "btn--ghost",
  danger:    "btn--danger",
};

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant = "secondary", size = "default", className, leading, trailing, children, type, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type ?? "button"}
      className={cx("btn", VARIANT_CLASS[variant], size === "xs" && "btn--xs", className)}
      {...rest}
    >
      {leading}
      {children}
      {trailing}
    </button>
  );
});
