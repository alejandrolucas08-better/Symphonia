import type { AuthUser, LoginPayload, RegisterPayload } from "../types/auth";

const ORIGIN = import.meta.env.VITE_API_BASE_URL ?? "";
const BASE = ORIGIN ? ORIGIN.replace(/\/$/, "") : "";

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<{ data: T; token?: string }> {
  const res = await fetch(`${BASE}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });

  const token = res.headers.get("Authorization")?.replace("Bearer ", "");

  const body = await res.json().catch(() => ({}));

  if (!res.ok) {
    const message =
      typeof body === "object" && body !== null && "error" in body
        ? String(body.error)
        : `Request failed (${res.status})`;
    throw new ApiError(res.status, message);
  }

  return { data: body as T, token: token ?? undefined };
}

const errorMessages: Record<string, string> = {
  "invalid request body": "Requisição inválida.",
  "name, email and password are required":
    "Preencha nome, email e senha.",
  "email already registered": "Este email já está cadastrado.",
  "invalid email format": "Formato de email inválido.",
  "password must be at least 8 characters":
    "A senha precisa ter pelo menos 8 caracteres.",
  "invalid email or password": "Email ou senha inválidos.",
  "email and password are required": "Preencha email e senha.",
  "missing authorization header": "Sessão expirada. Entre novamente.",
  "invalid or expired token": "Sessão expirada. Entre novamente.",
  "invalid authorization format": "Sessão expirada. Entre novamente.",
  unauthorized: "Sessão expirada. Entre novamente.",
};

export function userFacingError(err: unknown): string {
  if (err instanceof ApiError) {
    return errorMessages[err.message] ?? err.message;
  }
  return "Erro de conexão. Tente novamente.";
}

export const api = {
  async register(payload: RegisterPayload) {
    return request<AuthUser>("/api/register", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  async login(payload: LoginPayload) {
    return request<AuthUser>("/api/login", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  async session(token: string) {
    return request<AuthUser>("/api/session", {
      headers: { Authorization: `Bearer ${token}` },
    });
  },
};
