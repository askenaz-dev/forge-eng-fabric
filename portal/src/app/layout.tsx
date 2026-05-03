import "./globals.css";
import type { ReactNode } from "react";
import { Providers } from "./providers";

export const metadata = {
  title: "Forge Engineering Fabric",
  description: "Phase 0 portal",
};

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-neutral-50 text-neutral-900 dark:bg-neutral-950 dark:text-neutral-100">
        <Providers>
          <header className="border-b border-neutral-200 dark:border-neutral-800 px-6 py-3 flex items-center justify-between">
            <h1 className="text-lg font-semibold">Forge Engineering Fabric</h1>
            <a href="/api/auth/signout" className="text-sm underline opacity-70">sign out</a>
          </header>
          <main className="px-6 py-6">{children}</main>
        </Providers>
      </body>
    </html>
  );
}
