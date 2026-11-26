import { defineConfig } from "@playwright/test";
import { tmpdir } from "node:os";
import { join } from "node:path";

// Runs the real binary (with this UI embedded) against a throwaway config.
// Build first: npm run build && go build -o ../bin/minectl ../cmd/minectl
export const configDir = process.env.MINECTL_SMOKE_DIR ?? join(tmpdir(), "minectl-smoke");
const bin = process.env.MINECTL_BIN ?? "../bin/minectl";
const port = 18090;

export default defineConfig({
  testDir: "e2e",
  timeout: 60_000,
  workers: 1,
  reporter: process.env.CI ? "github" : "list",
  use: { baseURL: `http://127.0.0.1:${port}`, trace: "retain-on-failure" },
  webServer: {
    command: `rm -rf ${configDir} && mkdir -p ${configDir} && MINECTL_CONFIG_DIR=${configDir} ${bin} serve --listen 127.0.0.1:${port}`,
    url: `http://127.0.0.1:${port}/api/v1/health`,
    reuseExistingServer: false,
    timeout: 30_000,
  },
});
