import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const apiTarget = process.env["VITE_API_PROXY_TARGET"] ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    // 5173 is commonly taken by other Vite projects; fail loudly instead of drifting.
    port: 5180,
    strictPort: true,
    proxy: {
      "/api": apiTarget,
    },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    restoreMocks: true,
  },
});
