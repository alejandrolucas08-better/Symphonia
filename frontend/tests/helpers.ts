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

export type MockCallParticipant = {
  user_id: number;
  name: string;
  spoken_language: "PT-BR" | "EN-US" | "ES-ES" | "FR-FR";
  heard_language: "PT-BR" | "EN-US" | "ES-ES" | "FR-FR";
  joined_at: string;
};

export type MockCall = {
  id: number;
  code: string;
  host_user_id: number;
  status: "waiting" | "active" | "ended";
  participants: MockCallParticipant[];
  created_at: string;
  updated_at: string;
  ended_at: string | null;
};

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

const timestamp = "2026-01-01T00:00:00Z";

function seededCall(code: string): MockCall {
  return {
    id: code === "demo-room" ? 1 : 2,
    code,
    host_user_id: defaultUser.id,
    status: code === "demo-room" ? "active" : "waiting",
    participants: [
      {
        user_id: defaultUser.id,
        name: defaultUser.name,
        spoken_language: "PT-BR",
        heard_language: "EN-US",
        joined_at: timestamp,
      },
      ...(code === "demo-room"
        ? [
            {
              user_id: 2,
              name: "Mateus",
              spoken_language: "EN-US" as const,
              heard_language: "PT-BR" as const,
              joined_at: timestamp,
            },
          ]
        : []),
    ],
    created_at: timestamp,
    updated_at: timestamp,
    ended_at: null,
  };
}

export async function mockCallApi(
  page: Page,
  {
    user = defaultUser,
    calls = [seededCall("demo-room"), seededCall("equipe-42")],
  }: { user?: MockUser; calls?: MockCall[] } = {},
) {
  const state = new Map(
    calls.map((call) => [call.code, structuredClone(call)]),
  );
  let nextId = Math.max(0, ...calls.map((call) => call.id)) + 1;
  let nextCode = 1;

  await page.route("**/api/calls**", async (route) => {
    const request = route.request();
    if (request.headers().authorization !== `Bearer ${token}`) {
      return json(route, 401, { error: "unauthorized" });
    }

    const url = new URL(request.url());
    const parts = url.pathname.split("/").filter(Boolean);
    const code = parts[2] ? decodeURIComponent(parts[2]) : undefined;
    const action = parts[3];
    const payload = request.postDataJSON() as
      | {
          spoken_language: MockCallParticipant["spoken_language"];
          heard_language: MockCallParticipant["heard_language"];
        }
      | undefined;

    if (request.method() === "POST" && !code) {
      const canonicalCode = `sala-${String(nextCode++).padStart(3, "0")}`;
      const call: MockCall = {
        id: nextId++,
        code: canonicalCode,
        host_user_id: user.id,
        status: "waiting",
        participants: [
          {
            user_id: user.id,
            name: user.name,
            spoken_language: payload!.spoken_language,
            heard_language: payload!.heard_language,
            joined_at: timestamp,
          },
        ],
        created_at: timestamp,
        updated_at: timestamp,
        ended_at: null,
      };
      state.set(call.code, call);
      return json(route, 201, call);
    }

    const call = code ? state.get(code) : undefined;
    if (!call) return json(route, 404, { error: "call not found" });
    if (request.method() === "GET") return json(route, 200, call);

    if (action === "join" && request.method() === "POST") {
      if (call.status === "ended")
        return json(route, 409, { error: "call has ended" });
      let participant = call.participants.find(
        (item) => item.user_id === user.id,
      );
      if (!participant && call.participants.length >= 2)
        return json(route, 409, { error: "call is full" });
      if (participant) {
        participant.spoken_language = payload!.spoken_language;
        participant.heard_language = payload!.heard_language;
      } else {
        participant = {
          user_id: user.id,
          name: user.name,
          spoken_language: payload!.spoken_language,
          heard_language: payload!.heard_language,
          joined_at: timestamp,
        };
        call.participants.push(participant);
      }
      call.status = call.participants.length === 2 ? "active" : "waiting";
      return json(route, 200, call);
    }

    if (action === "language" && request.method() === "PATCH") {
      const participant = call.participants.find(
        (item) => item.user_id === user.id,
      );
      if (!participant)
        return json(route, 403, { error: "you are not an active participant" });
      participant.spoken_language = payload!.spoken_language;
      participant.heard_language = payload!.heard_language;
      return json(route, 200, call);
    }

    if (action === "leave" && request.method() === "POST") {
      call.participants = call.participants.filter(
        (participant) => participant.user_id !== user.id,
      );
      call.status = "waiting";
      return json(route, 200, call);
    }

    if (action === "end" && request.method() === "POST") {
      if (call.host_user_id !== user.id)
        return json(route, 403, { error: "only the host can end this call" });
      call.status = "ended";
      call.ended_at = timestamp;
      return json(route, 200, call);
    }

    return json(route, 405, { error: "method not allowed" });
  });

  return state;
}
