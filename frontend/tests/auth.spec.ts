import { test, expect } from "@playwright/test";
import {
  defaultUser,
  mockAuthenticated,
  mockCallApi,
  mockLogin,
  mockRegister,
  mockVerifyEmail,
  token,
} from "./helpers";

const protectedRoutes = ["/home", "/call/demo-room/setup", "/call/demo-room"];

test.beforeEach(async ({ page }) => {
  await page.route("**/api/auth/providers", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ google: false }) }),
  );
});

test("cadastro: confirma o e-mail antes de entrar", async ({ page }) => {
  await mockRegister(page, {
    user: { ...defaultUser, name: "Bruno Lima", email: "bruno@example.com" },
  });
  await page.goto("/register");
  await page.getByLabel("nome", { exact: true }).fill("Bruno Lima");
  await page.getByLabel("email", { exact: true }).fill("bruno@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByLabel("confirmar senha").fill("secret-123");
  await page.getByRole("button", { name: "Criar conta" }).click();
  await expect(page.getByRole("heading", { name: "Confira seu e-mail" })).toBeVisible();
  await mockVerifyEmail(page, { ...defaultUser, name: "Bruno Lima", email: "bruno@example.com" });
  await page.goto("/verify-email?token=valid-test-token");
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

test("convite retorna à configuração da sala depois do login", async ({ page }) => {
  await mockLogin(page);
  await mockCallApi(page);
  await page.goto("/call/equipe-42/setup");
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("email", { exact: true }).fill("ana@example.com");
  await page.getByLabel("senha", { exact: true }).fill("secret-123");
  await page.getByRole("button", { name: "Entrar", exact: true }).click();
  await expect(page).toHaveURL(/\/call\/equipe-42\/setup$/);
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
  await expect
    .poll(() => page.evaluate(() => localStorage.getItem("symphonia_token")))
    .toBe(token);
  expect(
    await page.evaluate(() => sessionStorage.getItem("symphonia_token")),
  ).toBeNull();
});

test("sessão: token inválido redireciona para login", async ({ page }) => {
  await page.addInitScript((tkn) => {
    localStorage.setItem("symphonia_token", tkn);
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

test("recuperação solicita link sem revelar existência da conta e redefine a senha", async ({ page }) => {
  await page.route("**/api/auth/forgot-password", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "sent" }) }));
  await page.route("**/api/auth/reset-password", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ message: "password changed" }) }));
  await page.goto("/forgot-password");
  await page.getByLabel("e-mail").fill("ana@example.com");
  await page.getByRole("button", { name: "Enviar link" }).click();
  await expect(page.getByRole("status")).toContainText("Se este e-mail tiver uma conta");
  await page.goto("/reset-password?token=reset-test-token");
  await expect(page).toHaveURL(/\/reset-password$/);
  await page.getByLabel("nova senha", { exact: true }).fill("new-secret-123");
  await page.getByLabel("confirmar nova senha").fill("new-secret-123");
  await page.getByRole("button", { name: "Salvar senha" }).click();
  await expect(page.getByRole("status")).toContainText("Senha atualizada");
});

test("troca de senha mantém a sessão atual e atualiza o token", async ({ page }) => {
  await mockAuthenticated(page);
  await page.route("**/api/auth/change-password", (route) => route.fulfill({ status: 200, contentType: "application/json", headers: { Authorization: "Bearer renewed-token" }, body: JSON.stringify({ message: "password changed" }) }));
  await page.goto("/account/password");
  await page.getByLabel("senha atual").fill("old-secret-123");
  await page.getByLabel("nova senha", { exact: true }).fill("new-secret-123");
  await page.getByLabel("confirmar nova senha").fill("new-secret-123");
  await page.getByRole("button", { name: "Salvar senha" }).click();
  await expect(page.getByRole("status")).toHaveText("Senha alterada com sucesso.");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("symphonia_token"))).toBe("renewed-token");
});

test("Google só aparece quando configurado", async ({ page }) => {
  await page.route("**/api/auth/providers", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ google: true }) }));
  await page.goto("/login");
  await expect(page.getByRole("button", { name: "Continuar com Google" })).toBeVisible();
});

test("conta existente pode vincular Google com retorno à conta", async ({ page }) => {
  await mockAuthenticated(page);
  await page.route("**/api/auth/providers", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ google: true }) }));
  await page.route("**/api/auth/google/link/start", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ url: `${new URL(route.request().url()).origin}/auth/google/complete?ticket=linked-test-ticket&return_to=%2Faccount%2Fpassword` }) }));
  await page.route("**/api/auth/google/finish", (route) => route.fulfill({ status: 200, contentType: "application/json", headers: { Authorization: "Bearer linked-session" }, body: JSON.stringify({ ...defaultUser, google_linked: true }) }));
  await page.goto("/account/password");
  await page.getByRole("button", { name: "Vincular Google" }).click();
  await expect(page).toHaveURL(/\/account\/password$/);
  await expect(page.getByText("Google vinculado")).toBeVisible();
});
