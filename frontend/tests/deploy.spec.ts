import { test, expect } from "@playwright/test";

test.skip(!process.env.DEPLOY_E2E, "requires the production image");

test("production serves health, SPA routes, assets and the audio worklet", async ({ page, request }) => {
  const health = await request.get("/healthz");
  expect(health.status()).toBe(200);
  expect((await health.json()).status).toBe("ok");
  for (const path of ["/", "/login", "/register", "/call/abcd1234/setup"]) {
    const response = await request.get(path);
    expect(response.status()).toBe(200);
    expect(response.headers()["content-type"]).toContain("text/html");
  }
  const worklet = await request.get("/audio-capture-worklet.js");
  expect(worklet.status()).toBe(200);
  expect(worklet.headers()["content-type"]).toMatch(/javascript/);
  expect((await request.get("/assets/missing.js")).status()).toBe(404);
  expect((await request.get("/api/missing")).status()).toBe(404);
  expect((await request.get("/.env")).status()).toBe(404);
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/login");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page.reload();
  await expect(page.getByLabel("email", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => window.isSecureContext)).toBeTruthy();
  expect(errors).toEqual([]);
});
