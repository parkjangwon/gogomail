import type { NextConfig } from "next";

const isProduction = process.env.NODE_ENV === "production";
const scriptSrc = isProduction
  ? "script-src 'self' 'unsafe-inline'"
  : "script-src 'self' 'unsafe-inline' 'unsafe-eval'";

const config: NextConfig = {
  reactStrictMode: true,
  // Workspace package @gogomail/api-types exports raw .ts files; Turbopack
  // requires explicit transpile config to consume them in production builds.
  transpilePackages: ["@gogomail/api-types"],
  typescript: {
    tsconfigPath: "./tsconfig.json",
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
          key: "Content-Security-Policy",
          value: [
            "default-src 'self'",
            scriptSrc,
            "style-src 'self' 'unsafe-inline'",
            "img-src 'self' data: blob:",
            "connect-src 'self'",
            "font-src 'self' data:",
            "frame-src 'none'",
            "frame-ancestors 'none'",
            "object-src 'none'",
            "base-uri 'self'",
            "form-action 'self'",
            ...(isProduction ? ["upgrade-insecure-requests"] : []),
          ].join("; "),
        },
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
