import path from "node:path";
import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// Gateway lokal. Semua /api diproksikan supaya cookie sesi (HttpOnly) berlaku same-origin saat dev.
const GATEWAY = process.env.VITE_GATEWAY_URL ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "./src") },
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
