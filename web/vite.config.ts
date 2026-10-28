import fs from "node:fs";
import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { parse } from "yaml";
import { defineConfig, type Plugin } from "vitest/config";

// Gateway lokal. Semua /api diproksikan supaya cookie sesi (HttpOnly) berlaku same-origin saat dev.
const GATEWAY = process.env.VITE_GATEWAY_URL ?? "http://localhost:8080";

const OPENAPI_FILE = path.resolve(__dirname, "../api/openapi.yml");

/**
 * Halaman docs dibangun dari api/openapi.yml (satu sumber kebenaran). YAML di-parse saat build/dev dan
 * dikirim sebagai JSON, jadi browser tidak perlu memuat parser YAML. File yang berubah memuat ulang halaman.
 */
function openapi(): Plugin {
  const id = "virtual:openapi";
  const resolved = "\0" + id;
  return {
    name: "linkforge-openapi",
    resolveId: (source) => (source === id ? resolved : undefined),
    load(loadId) {
      if (loadId !== resolved) return;
      this.addWatchFile(OPENAPI_FILE);
      return `export default ${JSON.stringify(parse(fs.readFileSync(OPENAPI_FILE, "utf8")))};`;
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), openapi()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
  },
  test: {
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
  server: {
    port: 5173,
    proxy: {
      "/api": { target: GATEWAY, changeOrigin: true, ws: true },
      // Verifikasi password link hidup di root Gateway (POST /:alias/verify), bukan di /api
      "^/[A-Za-z0-9_-]+/verify$": { target: GATEWAY, changeOrigin: true },
    },
  },
});
