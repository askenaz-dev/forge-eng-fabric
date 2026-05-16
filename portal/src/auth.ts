import type { AuthOptions } from "next-auth";
import KeycloakProvider from "next-auth/providers/keycloak";

const keycloakIssuer = process.env.KEYCLOAK_ISSUER ?? "http://localhost:8080/realms/forge";
const keycloakClientId = process.env.KEYCLOAK_CLIENT_ID ?? "forge-portal";
const keycloakClientSecret = process.env.KEYCLOAK_CLIENT_SECRET ?? "";

// Workspace/tenant defaults applied at sign-in if the user has not yet picked.
// They are seeded into the JWT so server components and API routes can read
// them via getServerSession() without hitting an extra store.
const DEFAULT_TENANT_SLUG = process.env.PORTAL_DEFAULT_TENANT ?? "acme";
const DEFAULT_WORKSPACE_SLUG = process.env.PORTAL_DEFAULT_WORKSPACE ?? "engineering";

declare module "next-auth" {
  interface Session {
    accessToken?: string;
    error?: string;
    tenantSlug?: string;
    workspaceSlug?: string;
  }
}

declare module "next-auth/jwt" {
  interface JWT {
    accessToken?: string;
    refreshToken?: string;
    accessTokenExpires?: number;
    error?: string;
    tenantSlug?: string;
    workspaceSlug?: string;
  }
}

async function refreshAccessToken(token: any) {
  try {
    const body = new URLSearchParams({
      grant_type: "refresh_token",
      client_id: keycloakClientId,
      refresh_token: token.refreshToken,
    });
    if (keycloakClientSecret) {
      body.set("client_secret", keycloakClientSecret);
    }

    const response = await fetch(`${keycloakIssuer}/protocol/openid-connect/token`, {
      method: "POST",
      headers: { "content-type": "application/x-www-form-urlencoded" },
      body,
    });
    const refreshed = await response.json();
    if (!response.ok) throw refreshed;

    return {
      ...token,
      accessToken: refreshed.access_token,
      accessTokenExpires: Date.now() + refreshed.expires_in * 1000,
      refreshToken: refreshed.refresh_token ?? token.refreshToken,
      error: undefined,
    };
  } catch {
    return {
      ...token,
      accessToken: undefined,
      refreshToken: undefined,
      accessTokenExpires: 0,
      error: "RefreshAccessTokenError",
    };
  }
}

export const authOptions: AuthOptions = {
  providers: [
    KeycloakProvider({
      clientId: keycloakClientId,
      clientSecret: keycloakClientSecret,
      issuer: keycloakIssuer,
    }),
  ],
  session: { strategy: "jwt" },
  callbacks: {
    async jwt({ token, account, trigger, session }) {
      // Client-side session.update({ tenantSlug, workspaceSlug }) — re-signs
      // the JWT with the new active workspace. This is how the WorkspacePicker
      // and TenantPicker propagate switches without a separate cookie.
      if (trigger === "update" && session && typeof session === "object") {
        const s = session as { tenantSlug?: string; workspaceSlug?: string };
        if (typeof s.tenantSlug === "string") token.tenantSlug = s.tenantSlug;
        if (typeof s.workspaceSlug === "string") token.workspaceSlug = s.workspaceSlug;
        return token;
      }

      if (account?.access_token) {
        token.accessToken = account.access_token;
        token.accessTokenExpires = (account.expires_at ?? 0) * 1000;
        token.refreshToken = account.refresh_token;
        // Seed defaults on first sign-in so server components have a workspace
        // context before the user explicitly picks one.
        token.tenantSlug = token.tenantSlug ?? DEFAULT_TENANT_SLUG;
        token.workspaceSlug = token.workspaceSlug ?? DEFAULT_WORKSPACE_SLUG;
        return token;
      }

      if (Date.now() < ((token as any).accessTokenExpires ?? 0)) return token;
      if (!(token as any).refreshToken) return token;
      return refreshAccessToken(token);
    },
    async session({ session, token }) {
      session.accessToken = token.accessToken;
      session.error = token.error;
      session.tenantSlug = token.tenantSlug ?? DEFAULT_TENANT_SLUG;
      session.workspaceSlug = token.workspaceSlug ?? DEFAULT_WORKSPACE_SLUG;
      return session;
    },
  },
};
