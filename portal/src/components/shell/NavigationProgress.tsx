"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname, useSearchParams } from "next/navigation";

// NavigationProgress shows a thin top progress bar while the App Router
// transitions to a new route. It intercepts internal link clicks to start
// the bar immediately (the click happens before Next has begun rendering),
// then completes the bar when usePathname / useSearchParams change.
//
// Rationale: Next.js 14 App Router has no built-in router event API. We
// avoid a runtime dependency on `nprogress` by inlining the same idea.
export function NavigationProgress() {
  const pathname = usePathname();
  const search = useSearchParams();
  const [phase, setPhase] = useState<"idle" | "loading" | "done">("idle");
  const [progress, setProgress] = useState(0);
  const tickRef = useRef<number | null>(null);
  const lastUrlRef = useRef<string>("");

  // Intercept clicks on internal links and form submissions to start the bar.
  useEffect(() => {
    lastUrlRef.current = window.location.pathname + window.location.search;

    function start() {
      if (tickRef.current != null) window.clearInterval(tickRef.current);
      setPhase("loading");
      setProgress(8);
      // Trickle towards 90% so the bar feels alive even on slow navigations.
      tickRef.current = window.setInterval(() => {
        setProgress((p) => (p < 90 ? p + Math.max(1, (90 - p) * 0.1) : p));
      }, 200);
    }

    function isInternalNavigation(target: EventTarget | null): boolean {
      if (!(target instanceof Element)) return false;
      const anchor = target.closest("a");
      if (!anchor) return false;
      const href = anchor.getAttribute("href");
      if (!href) return false;
      if (href.startsWith("#") || href.startsWith("mailto:") || href.startsWith("tel:") || href.startsWith("javascript:")) return false;
      if (anchor.getAttribute("target") === "_blank") return false;
      if (anchor.hasAttribute("download")) return false;
      try {
        const url = new URL(href, window.location.href);
        if (url.origin !== window.location.origin) return false;
        const next = url.pathname + url.search;
        if (next === lastUrlRef.current) return false; // same URL — no nav
        return true;
      } catch {
        return false;
      }
    }

    function onClick(event: MouseEvent) {
      if (event.defaultPrevented) return;
      if (event.button !== 0) return; // only primary click
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (!isInternalNavigation(event.target)) return;
      start();
    }

    function onSubmit(event: SubmitEvent) {
      const form = event.target as HTMLFormElement | null;
      if (!form) return;
      // Only forms targeting same origin and not opening a new tab.
      if (form.target === "_blank") return;
      const action = form.getAttribute("action") ?? "";
      try {
        const url = new URL(action || window.location.href, window.location.href);
        if (url.origin !== window.location.origin) return;
      } catch {
        return;
      }
      start();
    }

    document.addEventListener("click", onClick);
    document.addEventListener("submit", onSubmit, true);
    return () => {
      document.removeEventListener("click", onClick);
      document.removeEventListener("submit", onSubmit, true);
      if (tickRef.current != null) window.clearInterval(tickRef.current);
    };
  }, []);

  // When the URL actually changes, finish the bar.
  useEffect(() => {
    const url = pathname + (search?.toString() ? `?${search}` : "");
    if (lastUrlRef.current === url) return;
    lastUrlRef.current = url;
    if (tickRef.current != null) {
      window.clearInterval(tickRef.current);
      tickRef.current = null;
    }
    setProgress(100);
    setPhase("done");
    const handle = window.setTimeout(() => {
      setPhase("idle");
      setProgress(0);
    }, 220);
    return () => window.clearTimeout(handle);
  }, [pathname, search]);

  if (phase === "idle") return null;
  return (
    <div className="nav-progress" aria-hidden>
      <div
        className="nav-progress-bar"
        style={{
          width: `${progress}%`,
          opacity: phase === "done" ? 0 : 1,
        }}
      />
    </div>
  );
}
