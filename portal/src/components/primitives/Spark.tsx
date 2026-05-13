// SVG sparkline used by the KPI cards. Pure SSR-safe component.

type SparkProps = {
  data: number[];
  color?: string;
  width?: number;
  height?: number;
  className?: string;
};

export function Spark({
  data,
  color = "var(--primary)",
  width = 110,
  height = 36,
  className = "spark",
}: SparkProps) {
  if (data.length < 2) return null;
  const p = 4;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  const pts = data.map((v, i) => {
    const x = p + (i * (width - p * 2)) / (data.length - 1);
    const y = height - p - ((v - min) / span) * (height - p * 2);
    return `${x},${y}`;
  });
  const area = `M ${pts[0]} L ${pts.join(" ")} L ${width - p},${height} L ${p},${height} Z`;
  const line = `M ${pts.join(" L ")}`;
  return (
    <svg className={className} viewBox={`0 0 ${width} ${height}`} aria-hidden="true">
      <path d={area} fill={color} opacity="0.10" />
      <path d={line} stroke={color} strokeWidth="1.5" fill="none" />
    </svg>
  );
}
