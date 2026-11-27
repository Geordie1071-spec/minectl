import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The build is embedded into the Go binary (internal/web). During
// development, run `minectl serve` and proxy the API to it.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: "../internal/web/dist",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": { target: process.env.MINECTL_API ?? "http://127.0.0.1:8080" },
    },
  },
});
