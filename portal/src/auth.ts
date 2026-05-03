import type { AuthOptions } from "next-auth";
import KeycloakProvider from "next-auth/providers/keycloak";

const keycloakIssuer = process.env.KEYCLOAK_ISSUER ?? "http://localhost:8080/realms/forge";
const keycloakClientId = process.env.KEYCLOAK_CLIENT_ID ?? "forge-portal";
const keycloakClientSecret = process.env.KEYCLOAK_CLIENT_SECRET ?? "";

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
    return { ...token, error: "RefreshAccessTokenError" };
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
    async jwt({ token, account }) {
      if (account?.access_token) {
        token.accessToken = account.access_token;
        token.accessTokenExpires = (account.expires_at ?? 0) * 1000;
        token.refreshToken = account.refresh_token;
        return token;
      }
      if (Date.now() < ((token as any).accessTokenExpires ?? 0)) return token;
      if (!(token as any).refreshToken) return token;
      return refreshAccessToken(token);
    },
    async session({ session, token }) {
      (session as any).accessToken = token.accessToken;
      (session as any).error = (token as any).error;
      return session;
    },
  },
};
