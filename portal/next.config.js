/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  experimental: { instrumentationHook: true, typedRoutes: true },
};
module.exports = nextConfig;
