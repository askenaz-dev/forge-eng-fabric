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
  webpack: (config, { dev }) => {
    if (dev) {
      config.watchOptions = {
        ...config.watchOptions,
        ignored: [
          ...(Array.isArray(config.watchOptions?.ignored) ? config.watchOptions.ignored : []),
          "**/node_modules/**",
          "**/.git/**",
          "**/.next/**",
          "C:/DumpStack.log.tmp",
          "C:/hiberfil.sys",
          "C:/pagefile.sys",
          "C:/swapfile.sys",
        ],
      };
    }
    return config;
  },
};
module.exports = nextConfig;
