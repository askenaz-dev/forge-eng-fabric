import type { ComponentType, SVGProps, ReactNode } from "react";
import { Spark } from "./Spark";

export type KpiDelta = {
  dir: "up" | "down";
  v: string;
};

export type KpiProps = {
  label: ReactNode;
  icon?: ComponentType<SVGProps<SVGSVGElement>>;
  num: string;
  unit?: string;
  delta?: KpiDelta;
  foot?: ReactNode;
  data?: number[];
  color?: string;
};

export function Kpi({ label, icon: Icon, num, unit, delta, foot, data, color }: KpiProps) {
  return (
    <div className="kpi">
      <div className="lbl">
        {Icon && <Icon />}
        <span>{label}</span>
      </div>
      <div className="num">
        {num}
        {unit && <small> {unit}</small>}
      </div>
      <div className="foot">
        {delta && (
          <span className={`delta ${delta.dir === "up" ? "up" : "down"}`}>
            {delta.dir === "up" ? "▲" : "▼"} {delta.v}
          </span>
        )}
        {foot && <span>{foot}</span>}
      </div>
      {data && data.length > 1 && <Spark data={data} color={color} />}
    </div>
  );
}

export function KpiSkeleton() {
  return (
    <div className="kpi">
      <div className="skeleton" style={{ width: 90, height: 12 }} />
      <div className="skeleton" style={{ width: 120, height: 36, marginTop: 10 }} />
      <div className="skeleton" style={{ width: 60, height: 10, marginTop: 12 }} />
    </div>
  );
}
