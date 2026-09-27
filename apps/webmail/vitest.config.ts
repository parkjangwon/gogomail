import { defineConfig } from "vitest/config";
import path from "path";

// Webmail unit tests target pure, framework-agnostic logic modules (keyboard
// focus-guard, bulk-selection). They do not render React, so a lightweight
// `node` environment is sufficient — no jsdom/testing-library required.
export default defineConfig({
  test: {
    globals: true,
    environment: "node",
    setupFiles: [],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    exclude: ["node_modules/**", "e2e/**", ".next/**", "dist/**"],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
