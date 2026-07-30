import path from "node:path";

import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

const projectDirectory = import.meta.dirname;

export default defineConfig({
  plugins: [
    react({
      jsxRuntime: "automatic",
    }),
  ],
  resolve: {
    alias: {
      "@": path.resolve(projectDirectory, "./src"),
    },
  },
  envPrefix: ["APP_"],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        changeOrigin: true,
        target: "http://localhost:8080",
      },
      "/healthz": {
        changeOrigin: true,
        target: "http://localhost:8080",
      },
    },
  },
});
