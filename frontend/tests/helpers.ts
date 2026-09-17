import type { Page, Route, WebSocketRoute } from "@playwright/test";
import { CALL_EVENT } from "../src/types/realtime";

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

export async function installMediaMocks(
  page: Page,
  errorName?: "NotAllowedError" | "NotFoundError" | "NotReadableError" | "OverconstrainedError",
) {
  await page.addInitScript((failure) => {
    const tracks: Array<{
      enabled: boolean;
      readyState: "live" | "ended";
      stop: () => void;
    }> = [];
    Object.defineProperty(window, "__testAudioTracks", {
      configurable: true,
      value: tracks,
    });
    Object.defineProperty(window, "__testPlaybackSampleRates", {
      configurable: true,
      value: [] as number[],
    });
    Object.defineProperty(navigator, "mediaDevices", {
      configurable: true,
      value: {
        enumerateDevices: async () => [
          {
            deviceId: "test-microphone",
            groupId: "test-group",
            kind: "audioinput",
            label: "Microfone de teste",
            toJSON: () => ({}),
          },
        ],
        getUserMedia: async () => {
          if (failure) throw new DOMException("test media failure", failure);
          const track = {
            enabled: true,
            readyState: "live" as const,
            stop() {
              (this as { readyState: string }).readyState = "ended";
            },
          };
          tracks.push(track);
          return {
            getTracks: () => [track],
            getAudioTracks: () => [track],
          };
        },
      },
    });

    class FakeAudioNode {
      connect() {
        return this;
      }
      disconnect() {}
    }
    class FakeGainNode extends FakeAudioNode {
      gain = { value: 1, setValueAtTime(value: number) { this.value = value; } };
    }
    class FakeBufferSource extends FakeAudioNode {
      buffer: unknown;
      onended: (() => void) | null = null;
      start() {}
      stop() { this.onended?.(); }
    }
    class FakeAudioContext {
      state: AudioContextState = "running";
      currentTime = 1;
      destination = new FakeAudioNode();
      audioWorklet = { addModule: async () => undefined };
      createMediaStreamSource() { return new FakeAudioNode(); }
      createGain() { return new FakeGainNode(); }
      createBufferSource() { return new FakeBufferSource(); }
      createBuffer(_channels: number, length: number, sampleRate: number) {
        (window as unknown as { __testPlaybackSampleRates: number[] })
          .__testPlaybackSampleRates.push(sampleRate);
        const data = new Float32Array(length);
        return { getChannelData: () => data };
      }
      async resume() { this.state = "running"; }
      async close() { this.state = "closed"; }
    }
    class FakeAudioWorkletNode extends FakeAudioNode {
      port = { onmessage: null as ((event: MessageEvent<Int16Array>) => void) | null };
      constructor() {
        super();
        Object.defineProperty(window, "__emitAudioSamples", {
          configurable: true,
          value: () => this.port.onmessage?.(
            { data: new Int16Array(320).fill(1200) } as MessageEvent<Int16Array>,
          ),
        });
      }
    }
    Object.defineProperty(window, "AudioContext", { configurable: true, value: FakeAudioContext });
    Object.defineProperty(window, "AudioWorkletNode", { configurable: true, value: FakeAudioWorkletNode });
  }, errorName);
}

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

export type MockCallSocket = {
  send: (type: string, data: unknown, requestId?: string) => void;
  sendRaw: (message: string) => void;
  closeAbnormally: () => Promise<void>;
  connectionCount: () => number;
  protocols: () => string[];
  binaryFrames: () => number;
  sendBinary: (payload: number[]) => void;
};

const socketMocks = new WeakMap<Page, MockCallSocket>();

export function getCallSocketMock(page: Page): MockCallSocket {
  const mock = socketMocks.get(page);
  if (!mock) throw new Error("Call WebSocket mock has not been installed");
  return mock;
}

async function mockCallSocket(
  page: Page,
  state: Map<string, MockCall>,
  user: MockUser,
) {
  const sockets = new Set<WebSocketRoute>();
  let connections = 0;
  let messageId = 0;
  let requestedProtocols: string[] = [];
  let binaryFrameCount = 0;
  const envelope = (type: string, data: unknown, requestId?: string) =>
    JSON.stringify({
      version: 1,
      type,
      ...(requestId ? { request_id: requestId } : {}),
      data,
      occurred_at: timestamp,
    });

  await page.routeWebSocket("**/api/calls/*/ws", (socket) => {
    sockets.add(socket);
    connections += 1;
    requestedProtocols = socket.protocols();
    const parts = new URL(socket.url()).pathname.split("/").filter(Boolean);
    const call = state.get(decodeURIComponent(parts[2] ?? ""));

    socket.onMessage((raw) => {
      if (typeof raw !== "string") {
        binaryFrameCount += 1;
        socket.send(raw);
        return;
      }
      if (!call) return;
      const incoming = JSON.parse(raw) as {
        type: string;
        request_id: string;
        data: { text?: string; muted?: boolean };
      };
      if (incoming.type === CALL_EVENT.JOIN_CALL) {
        socket.send(
          envelope(
            CALL_EVENT.JOIN_CALL,
            {
              call,
              mute_states: call.participants.map((participant) => ({
                user_id: participant.user_id,
                muted: false,
              })),
            },
            incoming.request_id,
          ),
        );
      } else if (
        incoming.type === CALL_EVENT.MESSAGE &&
        typeof incoming.data.text === "string"
      ) {
        const participant =
          call.participants.find((item) => item.user_id === user.id) ??
          call.participants[0];
        messageId += 1;
        socket.send(
          envelope(CALL_EVENT.MESSAGE, {
            id: messageId,
            user_id: participant.user_id,
            name: participant.name,
            language: participant.spoken_language,
            text: incoming.data.text,
            sent_at: timestamp,
          }),
        );
      } else if (
        incoming.type === CALL_EVENT.MUTE_STATE &&
        typeof incoming.data.muted === "boolean"
      ) {
        socket.send(
          envelope(CALL_EVENT.MUTE_STATE, {
            user_id: user.id,
            muted: incoming.data.muted,
          }),
        );
      }
    });
    socket.onClose(() => sockets.delete(socket));
  });

  socketMocks.set(page, {
    send: (type, data, requestId) => {
      const message = envelope(type, data, requestId);
      sockets.forEach((socket) => socket.send(message));
    },
    sendRaw: (message) => sockets.forEach((socket) => socket.send(message)),
    closeAbnormally: async () => {
      await Promise.all(
        [...sockets].map((socket) =>
          socket.close({ code: 1011, reason: "test reconnect" }),
        ),
      );
      sockets.clear();
    },
    connectionCount: () => connections,
    protocols: () => requestedProtocols,
    binaryFrames: () => binaryFrameCount,
    sendBinary: (payload) => {
      const message = Buffer.from(payload);
      sockets.forEach((socket) => socket.send(message));
    },
  });
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

  await mockCallSocket(page, state, user);

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
