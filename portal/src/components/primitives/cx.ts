// Lightweight className composer. We don't depend on clsx at runtime to keep
// SSR-only primitives free of "use client" boundaries. clsx is still listed
// in package.json for use inside client components that want its
// object-conditional API.

export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}
