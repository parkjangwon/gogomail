import type { NextConfig } from "next";

const config: NextConfig = {
  reactStrictMode: true,
  typescript: {
    tsconfigPath: "./tsconfig.json",
  },
  experimental: {
    optimizePackageImports: ["@cloudscape-design/components"],
  },
  env: {
    NEXT_PUBLIC_GOGOMAIL_BACKEND_URL:
      process.env.GOGOMAIL_BACKEND_URL || "http://localhost:8080",
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
        { key: "X-Frame-Options", value: "SAMEORIGIN" },
        { key: "X-XSS-Protection", value: "1; mode=block" },
      ],
    },
  ],
};

export default config;
