import { HTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export type TerminalProps = HTMLAttributes<HTMLDivElement> & {
  title?: string;
  children: ReactNode;
};

export function Terminal({ title = "terminal", className, children, ...rest }: TerminalProps) {
  return (
    <div className={cx("terminal", className)} {...rest}>
      <div className="tbar">
        <span>{title}</span>
      </div>
      <div className="body">{children}</div>
    </div>
  );
}

export function Code({ className, ...rest }: HTMLAttributes<HTMLPreElement>) {
  return <pre className={cx("code", className)} {...rest} />;
}
