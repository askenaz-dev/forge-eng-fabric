/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  experimental: {
    instrumentationHook: true,
    // `typedRoutes` is disabled: the Portal has many query-string URLs
    // (e.g. `/assets?kind=agent`, `/?run=<id>`) that the static checker
    // can't validate. We rely on integration tests instead.
    typedRoutes: false,
  },
};
module.exports = nextConfig;
