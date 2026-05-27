import type { NextConfig } from "next";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const appRoot = dirname(fileURLToPath(import.meta.url));
const repoRoot = resolve(appRoot, "../..");

const config: NextConfig = {
  reactStrictMode: true,
  // Workspace package @gogomail/api-types exports raw .ts files; Turbopack
  // requires explicit transpile config to consume them in production builds.
  transpilePackages: ["@gogomail/api-types"],
  typescript: {
    tsconfigPath: "./tsconfig.json",
  },
  turbopack: {
    root: repoRoot,
  },
  experimental: {
    optimizePackageImports: ["@cloudscape-design/components"],
  },
  generateBuildId: async () => {
    return process.env.GIT_SHA || `dev-${Date.now()}`;
  },
  // output: 'standalone',  // Uncomment for Docker deployments
  headers: async () => [
    {
      source: "/:path*",
      headers: [
        { key: "X-Content-Type-Options", value: "nosniff" },
        { key: "X-Frame-Options", value: "DENY" },
        { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
        { key: "Cross-Origin-Resource-Policy", value: "same-origin" },
        { key: "X-DNS-Prefetch-Control", value: "off" },
        {
          key: "Strict-Transport-Security",
          value: "max-age=63072000; includeSubDomains; preload",
        },
        { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
        { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
      ],
    },
  ],
};

export default config;
