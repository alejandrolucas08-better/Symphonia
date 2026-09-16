import type { Page, Route } from "@playwright/test";

export type MockUser = {
  id: number;
  name: string;
  email: string;
  created_at: string;
};

export const defaultUser: MockUser = {
  id: 1,
  name: "Ana Souza",
  email: "ana@example.com",
  created_at: "2026-01-01T00:00:00Z",
};

export const token = "test-token";

function json(
  route: Route,
  status: number,
  body: unknown,
  headers?: Record<string, string>,
) {
  return route.fulfill({
    status,
    contentType: "application/json",
    headers,
    body: JSON.stringify(body),
  });
}

export async function mockAuthenticated(
  page: Page,
  user: MockUser = defaultUser,
) {
  await page.addInitScript((tkn) => {
    sessionStorage.setItem("symphonia_token", tkn);
  }, token);
  await page.route("**/api/session", (route) =>
    json(route, 200, user, { Authorization: `Bearer ${token}` }),
  );
}

export async function mockUnauthenticated(page: Page) {
  await page.route("**/api/session", (route) =>
    json(route, 401, { error: "sessão inválida" }),
  );
}

export async function mockRegister(
  page: Page,
  {
    status = 201,
    user = defaultUser,
    error,
  }: { status?: number; user?: MockUser; error?: string } = {},
) {
  await page.route("**/api/register", (route) =>
    json(route, status, error ? { error } : user, {
      Authorization: `Bearer ${token}`,
    }),
  );
}

export async function mockLogin(
  page: Page,
  {
    status = 200,
    user = defaultUser,
    error,
  }: { status?: number; user?: MockUser; error?: string } = {},
) {
  await page.route("**/api/login", (route) =>
    json(route, status, error ? { error } : user, {
      Authorization: `Bearer ${token}`,
    }),
  );
}