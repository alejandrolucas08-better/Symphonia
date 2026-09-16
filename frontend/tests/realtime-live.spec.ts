import { expect, test, type APIRequestContext } from "@playwright/test";

const apiOrigin = process.env.LIVE_API_ORIGIN ?? "http://127.0.0.1:8080";

test.skip(!process.env.LIVE_E2E, "requires the real backend and PostgreSQL");

async function register(
  request: APIRequestContext,
  name: string,
  email: string,
) {
  const response = await request.post(`${apiOrigin}/api/register`, {
    data: { name, email, password: "test-only-123" },
  });
  expect(response.status()).toBe(201);
  const authorization = response.headers().authorization;
  expect(authorization).toMatch(/^Bearer /);
  return authorization.replace(/^Bearer /, "");
}

test("two real sessions exchange call events", async ({ browser, request }) => {
  const suffix = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
  const hostToken = await register(
    request,
    "Ana Tempo Real",
    `ana-${suffix}@example.com`,
  );
  const guestToken = await register(
    request,
    "Bia Tempo Real",
    `bia-${suffix}@example.com`,
  );

  const created = await request.post(`${apiOrigin}/api/calls`, {
    headers: { Authorization: `Bearer ${hostToken}` },
    data: { spoken_language: "PT-BR", heard_language: "EN-US" },
  });
  expect(created.status()).toBe(201);
  const call = (await created.json()) as { code: string };
  const joined = await request.post(
    `${apiOrigin}/api/calls/${call.code}/join`,
    {
      headers: { Authorization: `Bearer ${guestToken}` },
      data: { spoken_language: "EN-US", heard_language: "PT-BR" },
    },
  );
  expect(joined.status()).toBe(200);

  const contextOptions = {
    baseURL: "http://127.0.0.1:5173",
    permissions: ["microphone"] as "microphone"[],
  };
  const hostContext = await browser.newContext(contextOptions);
  const guestContext = await browser.newContext(contextOptions);
  await hostContext.addInitScript((token) => {
    sessionStorage.setItem("symphonia_token", token);
  }, hostToken);
  await guestContext.addInitScript((token) => {
    sessionStorage.setItem("symphonia_token", token);
  }, guestToken);

  const host = await hostContext.newPage();
  const guest = await guestContext.newPage();
  await host.goto(`/call/${call.code}?audioDiagnostics=1`);
  await expect(host.getByText("conectado").first()).toBeVisible();
  await guest.goto(`/call/${call.code}?audioDiagnostics=1`);
  await expect(guest.getByText("conectado").first()).toBeVisible();
  const diagnostics = (page: typeof host) =>
    page.evaluate(
      () =>
        (window as Window & {
          __symphoniaAudioDiagnostics?: {
            outgoingFrames: number;
            incomingFrames: number;
            liveTracks: number;
          };
        }).__symphoniaAudioDiagnostics,
    );
  await expect.poll(async () => (await diagnostics(host))?.outgoingFrames ?? 0).toBeGreaterThan(0);
  await expect.poll(async () => (await diagnostics(guest))?.outgoingFrames ?? 0).toBeGreaterThan(0);
  await expect.poll(async () => (await diagnostics(host))?.incomingFrames ?? 0).toBeGreaterThan(0);
  await expect.poll(async () => (await diagnostics(guest))?.incomingFrames ?? 0).toBeGreaterThan(0);

  await host.getByLabel("Mensagem", { exact: true }).fill("Olá da Ana");
  await host.getByRole("button", { name: "Enviar mensagem" }).click();
  await expect(host.getByRole("log")).toContainText("Olá da Ana");
  await expect(guest.getByRole("log")).toContainText("Olá da Ana");

  await guest.getByLabel("Mensagem", { exact: true }).fill("Olá da Bia");
  await guest.getByRole("button", { name: "Enviar mensagem" }).click();
  await expect(host.getByRole("log")).toContainText("Olá da Bia");
  await expect(guest.getByRole("log")).toContainText("Olá da Bia");

  await host
    .getByRole("button", { name: "Desativar microfone", exact: true })
    .click();
  await host.waitForTimeout(300);
  const mutedFrames = (await diagnostics(host))?.outgoingFrames ?? 0;
  await host.waitForTimeout(500);
  expect((await diagnostics(host))?.outgoingFrames).toBe(mutedFrames);
  await expect(
    guest
      .locator(".participant-row")
      .filter({ hasText: "Ana Tempo Real" })
      .locator(".lucide-mic-off"),
  ).toBeVisible();
  await host
    .getByRole("button", { name: "Ativar microfone", exact: true })
    .click();
  await expect.poll(async () => (await diagnostics(host))?.outgoingFrames ?? 0).toBeGreaterThan(mutedFrames);

  await host.getByRole("button", { name: "Configurar idiomas" }).click();
  await host.getByLabel("eu falo").selectOption("ES-ES");
  await expect(
    guest.locator(".participant-row").filter({ hasText: "Ana Tempo Real" }),
  ).toContainText("ES-ES");

  await guest
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await guest
    .getByRole("dialog")
    .getByRole("button", { name: "Sair da chamada" })
    .click();
  await expect(guest).toHaveURL(/\/home$/);
  await expect.poll(async () => (await diagnostics(guest))?.liveTracks ?? -1).toBe(0);

  await host
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await host
    .getByRole("dialog")
    .getByRole("button", { name: "Encerrar chamada" })
    .click();
  await expect(host).toHaveURL(/\/home$/);

  await hostContext.close();
  await guestContext.close();
});
