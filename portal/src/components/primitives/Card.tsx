import { HTMLAttributes, ReactNode } from "react";
import { cx } from "./cx";

export type CardProps = HTMLAttributes<HTMLDivElement>;

export function Card({ className, ...rest }: CardProps) {
  return <div className={cx("card", className)} {...rest} />;
}

export type CardHeaderProps = {
  title: ReactNode;
  sub?: ReactNode;
  right?: ReactNode;
  className?: string;
};

export function CardHeader({ title, sub, right, className }: CardHeaderProps) {
  return (
    <div className={cx("card-hd", className)}>
      <div style={{ minWidth: 0 }}>
        <h3>{title}</h3>
        {sub && <div className="sub">{sub}</div>}
      </div>
      {right && <div className="right">{right}</div>}
    </div>
  );
}

export type CardBodyProps = HTMLAttributes<HTMLDivElement>;

export function CardBody({ className, style, ...rest }: CardBodyProps) {
  return <div className={className} style={{ padding: 14, ...style }} {...rest} />;
}
