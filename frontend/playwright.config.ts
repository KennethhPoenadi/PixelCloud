import { defineConfig, devices } from '@playwright/test'

// Runs against a running stack (`make up`); override with PIXELCLOUD_URL.
export default defineConfig({
  testDir: './e2e',
  timeout: 180_000,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'github' : 'list',
  use: {
    baseURL: process.env.PIXELCLOUD_URL ?? 'http://localhost:8080',
    trace: 'retain-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
