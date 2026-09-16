import { test, expect } from "@playwright/test";
import {
  defaultUser,
  mockAuthenticated,
  mockLogin,
  mockRegister,
  token,
} from "./helpers";

const protectedRoutes = ["/home", "/call/demo-room/setup", "/call/demo-room"];

test("cadastro: cria conta e aterrissa em /home", async ({ page }) => {
  await mockRegister(page, {
    user: { ...defaultUser, name: "Bruno Lima", email: "bruno@example.com" },
  });
  await page.goto("/register");
  await page.getByLabel("nome", { exact: true }).fill("Bruno Lima");
  await page.getByLabel("email", { exact: true }).fill("bruno@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByLabel("confirmar senha").fill("secret-123");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "BRUNO LIMA.",
  );
  await expect(page.locator(".account-name")).toHaveText("Bruno Lima");
});

test("cadastro: email duplicado é rejeitado", async ({ page }) => {
  await mockRegister(page, {
    status: 409,
    error: "email already registered",
  });
  await page.goto("/register");
  await page.getByLabel("nome", { exact: true }).fill("Bruno Lima");
  await page.getByLabel("email", { exact: true }).fill("bruno@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByLabel("confirmar senha").fill("secret-123");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "Este email já está cadastrado.",
  );
  await expect(page).toHaveURL(/\/register$/);
});

test("login: credenciais válidas entram em /home", async ({ page }) => {
  await mockLogin(page);
  await page.goto("/login");
  await page.getByLabel("email", { exact: true }).fill("ana@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/home$/);
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "ANA SOUZA.",
  );
  await expect(page.locator(".account-name")).toHaveText("Ana Souza");
});

test("login: credenciais inválidas exibem erro", async ({ page }) => {
  await mockLogin(page, { status: 401, error: "invalid email or password" });
  await page.goto("/login");
  await page.getByLabel("email", { exact: true }).fill("ana@example.com");
  await page.getByLabel("senha", { exact: true }).fill("senha-errada");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveText("Email ou senha inválidos.");
  await expect(page).toHaveURL(/\/login$/);
});

test("logout: encerra a sessão e volta ao início", async ({ page }) => {
  await mockLogin(page);
  await page.goto("/login");
  await page.getByLabel("email", { exact: true }).fill("ana@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/home$/);

  await page.getByRole("button", { name: "Sair" }).click();
  await expect(page).toHaveURL("/");

  await page.goto("/home");
  await expect(page).toHaveURL(/\/login$/);
});

test("sessão: token válido restaura autenticação ao recarregar", async ({
  page,
}) => {
  await mockAuthenticated(page);
  await page.goto("/home");
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "ANA SOUZA.",
  );
  await page.reload();
  await expect(page.getByRole("heading", { level: 1 })).toContainText(
    "ANA SOUZA.",
  );
  await expect(page.locator(".account-name")).toHaveText("Ana Souza");
});

test("sessão: token inválido redireciona para login", async ({ page }) => {
  await page.addInitScript((tkn) => {
    sessionStorage.setItem("symphonia_token", tkn);
  }, token);
  await page.route("**/api/session", (route) =>
    route.fulfill({
      status: 401,
      contentType: "application/json",
      body: JSON.stringify({ error: "invalid or expired token" }),
    }),
  );
  await page.goto("/home");
  await expect(page).toHaveURL(/\/login$/);
});

test("rotas privadas exigem autenticação", async ({ page }) => {
  for (const route of protectedRoutes) {
    await page.goto(route);
    await expect(page).toHaveURL(/\/login$/);
  }
  await expect(page.getByLabel("email", { exact: true })).toBeVisible();
});