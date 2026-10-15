import type { AuthUser, LoginPayload, RegisterPayload } from "../types/auth";
import type { Call, CallLanguagePayload, CreateCallPayload } from "../types/call";

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
  "call not found": "Chamada não encontrada.",
  "call is full": "A chamada já está cheia.",
  "call full": "A chamada já está cheia.",
  "call has ended": "Esta chamada já foi encerrada.",
  "call ended": "Esta chamada já foi encerrada.",
  "only the host can end the call": "Apenas quem criou a chamada pode encerrá-la.",
  "only the host can end this call": "Apenas quem criou a chamada pode encerrá-la.",
  "only host can end call": "Apenas quem criou a chamada pode encerrá-la.",
  "participant not found": "Você não participa desta chamada.",
  "you are not an active participant": "Você não participa desta chamada.",
  "invalid language": "Idioma inválido.",
  "invalid languages": "Idiomas inválidos.",
  "spoken_language and heard_language must be supported languages":
    "Selecione idiomas válidos.",
  "name must contain 1 to 80 characters":
    "O nome da reunião deve ter entre 1 e 80 caracteres.",
  "email not verified": "Confirme seu e-mail antes de entrar.",
  "email delivery is unavailable": "O envio de e-mails está indisponível. Tente mais tarde.",
  "invalid or expired link": "Este link expirou ou já foi utilizado.",
  "password must be 8 to 72 bytes": "A senha deve ter entre 8 e 72 caracteres.",
  "current password is incorrect": "A senha atual está incorreta.",
  "account has no password": "Esta conta usa o Google. Defina uma senha pela recuperação de senha.",
};

export function userFacingError(err: unknown): string {
  if (err instanceof ApiError) {
    return errorMessages[err.message] ?? err.message;
  }
  return "Erro de conexão. Tente novamente.";
}

export const api = {
  async register(payload: RegisterPayload) {
    return request<{ message: string }>("/api/register", {
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

  async providers() {
    return request<{ google: boolean }>("/api/auth/providers");
  },

  async resendVerification(email: string) {
    return request<{ message: string }>("/api/auth/resend-verification", { method: "POST", body: JSON.stringify({ email }) });
  },

  async verifyEmail(token: string) {
    return request<AuthUser>("/api/auth/verify-email", { method: "POST", body: JSON.stringify({ token }) });
  },

  async forgotPassword(email: string) {
    return request<{ message: string }>("/api/auth/forgot-password", { method: "POST", body: JSON.stringify({ email }) });
  },

  async resetPassword(token: string, newPassword: string) {
    return request<{ message: string }>("/api/auth/reset-password", { method: "POST", body: JSON.stringify({ token, new_password: newPassword }) });
  },

  async changePassword(token: string, currentPassword: string, newPassword: string) {
    return request<{ message: string }>("/api/auth/change-password", {
      method: "POST", headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
    });
  },

  async finishGoogle(ticket: string) {
    return request<AuthUser>("/api/auth/google/finish", { method: "POST", body: JSON.stringify({ token: ticket }) });
  },

  async startGoogleLink(token: string) {
    return request<{ url: string }>("/api/auth/google/link/start", { method: "POST", headers: { Authorization: `Bearer ${token}` } });
  },

  async createCall(token: string, payload: CreateCallPayload) {
    return request<Call>("/api/calls", {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify(payload),
    });
  },

  async getCall(token: string, code: string) {
    return request<Call>(`/api/calls/${encodeURIComponent(code)}`, {
      headers: { Authorization: `Bearer ${token}` },
    });
  },

  async joinCall(token: string, code: string, payload: CallLanguagePayload) {
    return request<Call>(`/api/calls/${encodeURIComponent(code)}/join`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify(payload),
    });
  },

  async updateCallLanguage(
    token: string,
    code: string,
    payload: CallLanguagePayload,
  ) {
    return request<Call>(`/api/calls/${encodeURIComponent(code)}/language`, {
      method: "PATCH",
      headers: { Authorization: `Bearer ${token}` },
      body: JSON.stringify(payload),
    });
  },

  async leaveCall(token: string, code: string) {
    return request<Call>(`/api/calls/${encodeURIComponent(code)}/leave`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
  },

  async endCall(token: string, code: string) {
    return request<Call>(`/api/calls/${encodeURIComponent(code)}/end`, {
      method: "POST",
      headers: { Authorization: `Bearer ${token}` },
    });
  },
};
