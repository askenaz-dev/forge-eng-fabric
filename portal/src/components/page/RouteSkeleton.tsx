// Per-route skeleton primitives. Use these in `loading.tsx` files so each
// section presents a familiar shape while server components stream.
//
// Variants are intentionally lightweight: they communicate *layout intent*
// (list, split, gallery, single form, etc.) rather than every cell. The
// shimmer comes from `.route-skeleton-*` rules in globals.css.
import type { ReactNode } from "react";

export type RouteSkeletonVariant =
  | "default"
  | "dashboard"
  | "list"
  | "split"
  | "gallery"
  | "form"
  | "editor"
  | "detail";

export type RouteSkeletonProps = {
  variant?: RouteSkeletonVariant;
  label?: ReactNode;
};

export function RouteSkeleton({ variant = "default", label = "Loading…" }: RouteSkeletonProps) {
  return (
    <div className="route-skeleton" aria-busy="true" aria-live="polite" aria-label={typeof label === "string" ? label : undefined}>
      <div className="route-skeleton-meta">
        <span className="route-skeleton-loader">
          <span className="spinner" aria-hidden />
          <span>{label}</span>
        </span>
      </div>
      <div className="route-skeleton-row route-skeleton-row--eyebrow" />
      <div className="route-skeleton-row route-skeleton-row--title" />
      <div className="route-skeleton-row route-skeleton-row--sub" />
      <RouteSkeletonBody variant={variant} />
    </div>
  );
}

function RouteSkeletonBody({ variant }: { variant: RouteSkeletonVariant }) {
  switch (variant) {
    case "dashboard":
      return (
        <>
          <div className="route-skeleton-grid route-skeleton-grid--kpi">
            <div className="route-skeleton-card route-skeleton-card--kpi" />
            <div className="route-skeleton-card route-skeleton-card--kpi" />
            <div className="route-skeleton-card route-skeleton-card--kpi" />
            <div className="route-skeleton-card route-skeleton-card--kpi" />
          </div>
          <div className="route-skeleton-grid" style={{ gridTemplateColumns: "1.6fr 1fr" }}>
            <div className="route-skeleton-card route-skeleton-card--list" />
            <div className="route-skeleton-card route-skeleton-card--tall" />
          </div>
        </>
      );
    case "list":
      return (
        <div className="route-skeleton-grid" style={{ gridTemplateColumns: "1fr" }}>
          <div className="route-skeleton-card route-skeleton-card--list" />
        </div>
      );
    case "split":
      return (
        <div className="route-skeleton-grid route-skeleton-grid--split">
          <div className="route-skeleton-card route-skeleton-card--tall" />
          <div className="route-skeleton-card route-skeleton-card--list" />
        </div>
      );
    case "gallery":
      return (
        <div className="route-skeleton-grid">
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
        </div>
      );
    case "form":
      return (
        <div className="route-skeleton-grid" style={{ gridTemplateColumns: "1fr", maxWidth: 640 }}>
          <div className="route-skeleton-card route-skeleton-card--tall" />
        </div>
      );
    case "editor":
      return (
        <>
          <div style={{ display: "flex", gap: 8, marginBottom: 4 }}>
            <div className="route-skeleton-row route-skeleton-row--chip" />
            <div className="route-skeleton-row route-skeleton-row--chip" />
            <div className="route-skeleton-row route-skeleton-row--chip" />
          </div>
          <div className="route-skeleton-grid route-skeleton-grid--split">
            <div className="route-skeleton-card route-skeleton-card--tall" />
            <div className="route-skeleton-card route-skeleton-card--list" />
          </div>
        </>
      );
    case "detail":
      return (
        <>
          <div style={{ display: "flex", gap: 8 }}>
            <div className="route-skeleton-row route-skeleton-row--chip" />
            <div className="route-skeleton-row route-skeleton-row--chip" />
          </div>
          <div className="route-skeleton-grid">
            <div className="route-skeleton-card route-skeleton-card--tall" />
            <div className="route-skeleton-card route-skeleton-card--tall" />
            <div className="route-skeleton-card route-skeleton-card--tall" />
          </div>
        </>
      );
    default:
      return (
        <div className="route-skeleton-grid">
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
          <div className="route-skeleton-card" />
        </div>
      );
  }
}
