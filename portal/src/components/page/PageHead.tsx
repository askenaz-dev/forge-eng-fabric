import { ReactNode } from "react";

export type PageHeadProps = {
  eyebrow?: ReactNode;
  title: ReactNode;
  titleEm?: ReactNode;
  sub?: ReactNode;
  actions?: ReactNode;
};

export function PageHead({ eyebrow, title, titleEm, sub, actions }: PageHeadProps) {
  return (
    <div className="page-head">
      <div>
        {eyebrow && <div className="h-eyebrow">{eyebrow}</div>}
        <h1 className="page-title">
          {title} {titleEm && <em>{titleEm}</em>}
        </h1>
        {sub && <p className="page-sub">{sub}</p>}
      </div>
      {actions && <div className="page-meta">{actions}</div>}
    </div>
  );
}
