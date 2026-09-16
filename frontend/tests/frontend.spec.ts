import { test, expect } from "@playwright/test";

const routes = [
  "/",
  "/login",
  "/register",
  "/home",
  "/call/demo-room/setup",
  "/call/demo-room",
];

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(navigator.mediaDevices, "getUserMedia", {
      value: () => {
        throw new Error("Device access is forbidden in the demo");
      },
    });
  });
});

for (const route of routes) {
  test(`route ${route}: responsive, no errors, local assets only`, async ({
    page,
  }, info) => {
    const errors: string[] = [];
    const external: string[] = [];
    page.on("pageerror", (error) => errors.push(error.message));
    page.on("console", (message) => {
      if (["error", "warning"].includes(message.type()))
        errors.push(message.text());
    });
    page.on("request", (request) => {
      if (
        !request.url().startsWith("http://127.0.0.1:5173") &&
        !request.url().startsWith("data:")
      )
        external.push(request.url());
    });
    await page.goto(route);
    await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
    await page.evaluate(() => document.fonts.ready);
    await expect(page.locator("video, audio, iframe")).toHaveCount(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBeTruthy();
    if (route === "/") {
      for (const section of [
        "#sobre",
        "#como-funciona",
        "#demonstracao",
        ".final-cta",
      ]) {
        await page.locator(section).scrollIntoViewIfNeeded();
        await expect(
          page.locator(section).locator(".reveal").first(),
        ).toHaveClass(/revealed/);
      }
      await page.evaluate(() =>
        window.scrollTo({ top: 0, behavior: "instant" }),
      );
      await expect(page.locator(".demo-quote .translated")).toHaveCSS(
        "opacity",
        "1",
      );
    }
    if (route === "/call/demo-room")
      await expect(page.locator(".translated-text")).toHaveCSS("opacity", "1");
    await page.screenshot({
      path: info.outputPath("page.png"),
      fullPage: true,
      animations: "disabled",
    });
    expect(errors).toEqual([]);
    expect(external).toEqual([]);
  });
}

test("registration validation, create call, preferences, chat and end", async ({
  page,
}) => {
  await page.goto("/register");
  await page.getByLabel("nome", { exact: true }).fill("Alex Silva");
  await page.getByLabel("email", { exact: true }).fill("alex@example.com");
  await page.getByLabel("senha", { exact: true }).fill("test-only-123");
  await page.getByLabel("confirmar senha").fill("not-matching");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "As senhas precisam ser iguais.",
  );
  await page.getByLabel("confirmar senha").fill("test-only-123");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "ALEX SILVA.",
  );
  await page.getByRole("link", { name: "Criar chamada" }).click();
  await expect(page).toHaveURL(/\/call\/demo-room\/setup$/);
  await page.getByLabel("quero ouvir").selectOption("ES-ES");
  await page
    .getByRole("button", { name: "Desativar microfone", exact: true })
    .click();
  await page.getByRole("link", { name: "Entrar na chamada" }).click();
  await page.getByRole("button", { name: "Pausar simulação" }).click();
  await expect(
    page.getByRole("button", { name: "Ativar microfone", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  await page
    .getByRole("button", { name: "Ativar microfone", exact: true })
    .click();
  await expect(page.locator(".translated-text")).toHaveText(
    "¿Cómo va el proyecto?",
  );
  await page
    .getByRole("button", { name: "Silenciar som", exact: true })
    .click();
  await expect(page.getByLabel("Volume", { exact: true })).toHaveValue("0");
  await page.getByRole("button", { name: "Configurar idiomas" }).click();
  await expect(page.getByLabel("quero ouvir")).toHaveValue("ES-ES");
  await page.getByLabel("quero ouvir").selectOption("FR-FR");
  await expect(page.locator(".translated-text")).toHaveText(
    "Comment avance le projet ?",
  );
  await page.getByRole("button", { name: "Fechar idiomas" }).click();
  await page.getByLabel("Mensagem", { exact: true }).fill("Até a próxima!");
  await page.getByRole("button", { name: "Enviar mensagem" }).click();
  await expect(page.getByRole("log")).toContainText("Até a próxima!");
  await expect(page.getByLabel("Mensagem", { exact: true })).toHaveValue("");
  await page
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).not.toBeVisible();
  await page
    .getByRole("button", { name: "Encerrar chamada", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Encerrar chamada" })
    .click();
  await expect(page).toHaveURL(/\/home$/);
});

test("login, join validation and deep-link reload", async ({ page }) => {
  await page.goto("/login");
  await page.getByLabel("email", { exact: true }).fill("demo@example.com");
  await page.getByLabel("senha", { exact: true }).fill("demo-password");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/home$/);
  await page.getByLabel("Código da chamada").fill("bad/code");
  await page.getByRole("button", { name: "Entrar com código" }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.getByLabel("Código da chamada").fill(" equipe-42 ");
  await page.getByRole("button", { name: "Entrar com código" }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42\/setup$/);
  await page.reload();
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page.getByRole("link", { name: "Entrar na chamada" }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42$/);
});

test("reduced motion freezes ambient signal and preserves manual demo", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/");
  const canvas = page.locator("canvas");
  await expect
    .poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.width))
    .toBeGreaterThan(0);
  const before = await canvas.evaluate((node: HTMLCanvasElement) =>
    node.toDataURL(),
  );
  await page.getByRole("heading", { level: 1 }).focus();
  expect(
    await canvas.evaluate((node: HTMLCanvasElement) => node.toDataURL()),
  ).toBe(before);
  expect(
    await page
      .locator(".audio-wave span")
      .first()
      .evaluate((node) => getComputedStyle(node).animationName),
  ).toBe("none");
  await page.getByRole("button", { name: "Próxima fala" }).click();
  await expect(page.locator(".demo-quote")).toContainText(
    "Everything is going well.",
  );
  await expect(page.locator(".demo-quote .translated")).toHaveCSS(
    "opacity",
    "1",
  );
  await page.goto("/call/demo-room");
  await expect(page.locator(".translated-text")).toHaveCSS("opacity", "1");
  await page.getByRole("button", { name: "Próxima fala" }).click();
  await expect(page.locator(".original-text")).toHaveText(
    "Everything is going well.",
  );
});

test("signal renders pixels, animates, and can pause", async ({ page }) => {
  await page.goto("/");
  const canvas = page.locator("canvas");
  await expect
    .poll(() =>
      canvas.evaluate((node: HTMLCanvasElement) => {
        const pixels = node
          .getContext("2d")!
          .getImageData(0, 0, node.width, node.height).data;
        return pixels.filter((value, index) => index % 4 === 3 && value > 0)
          .length;
      }),
    )
    .toBeGreaterThan(1000);
  const frame = await canvas.evaluate((node: HTMLCanvasElement) =>
    node.toDataURL(),
  );
  await expect
    .poll(() => canvas.evaluate((node: HTMLCanvasElement) => node.toDataURL()))
    .not.toBe(frame);
  await page.getByRole("button", { name: "Pausar movimento" }).click();
  await expect(
    page.getByRole("button", { name: "Retomar movimento" }),
  ).toBeVisible();
});

test("keyboard navigation, mobile menu and unknown route", async ({
  page,
}, info) => {
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Pular para o conteúdo" }),
  ).toBeFocused();
  if (info.project.name === "mobile") {
    await page.getByRole("button", { name: "Abrir menu" }).click();
    await page
      .getByRole("link", { name: "como funciona", exact: true })
      .click();
    await expect(
      page.getByRole("button", { name: "Abrir menu" }),
    ).toHaveAttribute("aria-expanded", "false");
  }
  await page.goto("/does-not-exist");
  await expect(
    page.getByRole("heading", { name: "Página não encontrada." }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Voltar ao início" }).click();
  await expect(page).toHaveURL("/");
});

test("narrow layout stays within the viewport", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 740 });
  for (const route of routes) {
    await page.goto(route);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
      route,
    ).toBeTruthy();
    if (route === "/") {
      expect(await page.locator("#sobre").evaluate((node) => node.getBoundingClientRect().top)).toBeLessThan(740);
    }
  }
});

test("translation follows the original and speakers alternate", async ({
  page,
}) => {
  await page.clock.install();
  await page.goto("/call/demo-room");
  await expect(page.locator(".original-text")).toHaveText(
    "Como está o projeto?",
  );
  await expect(page.locator(".translated-text")).not.toHaveClass(
    /translation-ready/,
  );
  await page.clock.runFor(1400);
  await expect(page.locator(".translated-text")).toHaveClass(
    /translation-ready/,
  );
  await page.clock.runFor(6000);
  await expect(page.locator(".original-text")).toHaveText(
    "Everything is going well.",
  );
  await expect(page.locator(".speaker .participant-name")).toHaveText("Mateus");
});
