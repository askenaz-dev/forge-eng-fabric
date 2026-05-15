import { NextResponse, type NextRequest } from "next/server";
import { getToken } from "next-auth/jwt";

// When Keycloak's refresh-token call fails (e.g. Keycloak briefly unreachable,
// refresh token revoked, or session past its absolute lifetime), the next-auth
// jwt callback clears `accessToken` but the session cookie itself stays valid.
// Pages and server actions that then read `session.accessToken` end up with
// undefined and either silently render with no data or surface a confusing
// "missing access token" error from the app/error.tsx boundary.
//
// This middleware short-circuits that state: any request that lands with an
// expired/refresh-failed JWT gets bounced to `/api/auth/signin` so the user
// re-authenticates cleanly.
export async function middleware(req: NextRequest) {
  const token = await getToken({
    req,
    secret: process.env.NEXTAUTH_SECRET,
  });
  if (token && (token as { error?: string }).error === "RefreshAccessTokenError") {
    const url = new URL("/api/auth/signin", req.url);
    url.searchParams.set("callbackUrl", req.nextUrl.pathname + req.nextUrl.search);
    return NextResponse.redirect(url);
  }
  return NextResponse.next();
}

export const config = {
  // Skip next-auth's own routes (so the redirect target itself is reachable),
  // static assets, image optimisation, and the favicon. Everything else goes
  // through the refresh-failure check.
  matcher: ["/((?!api/auth|_next/static|_next/image|favicon.ico).*)"],
};
