// Root-level loading boundary. Next.js App Router shows this while a route's
// server components are streaming. We render a lightweight skeleton inside the
// existing PortalShell main area so the chrome (sidebar, topbar, breadcrumbs)
// stays visible during navigation.
import { RouteSkeleton } from "@/components/page/RouteSkeleton";

export default function Loading() {
  return <RouteSkeleton />;
}
