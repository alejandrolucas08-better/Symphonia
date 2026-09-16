import type { Call, CallParticipant } from "../types/call";
import type { LanguageCode } from "../types/demo";
import {
  CALL_EVENT,
  type CallEventType,
  type CallSocketStatus,
  type ClientCallEnvelope,
  type RealtimeMessage,
  type ServerCallEnvelope,
} from "../types/realtime";

type EventListener = (event: ServerCallEnvelope) => void;
type StatusListener = (status: CallSocketStatus) => void;
type BinaryListener = (payload: ArrayBuffer) => void;

const reconnectDelays = [500, 1000, 2000, 4000, 5000];
const terminalCloseCodes = new Set([1000, 4001, 4003, 4004]);
const maxAudioBufferedBytes = 64 * 1024;
const languageCodes = new Set<LanguageCode>([
  "PT-BR",
  "EN-US",
  "ES-ES",
  "FR-FR",
]);
const eventTypes = new Set<CallEventType>(Object.values(CALL_EVENT));

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isParticipant(value: unknown): value is CallParticipant {
  return (
    isRecord(value) &&
    typeof value.user_id === "number" &&
    typeof value.name === "string" &&
    languageCodes.has(value.spoken_language as LanguageCode) &&
    languageCodes.has(value.heard_language as LanguageCode) &&
    typeof value.joined_at === "string"
  );
}

function isCall(value: unknown): value is Call {
  return (
    isRecord(value) &&
    typeof value.id === "number" &&
    typeof value.code === "string" &&
    typeof value.host_user_id === "number" &&
    (value.status === "waiting" ||
      value.status === "active" ||
      value.status === "ended") &&
    Array.isArray(value.participants) &&
    value.participants.every(isParticipant) &&
    typeof value.created_at === "string" &&
    typeof value.updated_at === "string" &&
    (value.ended_at === null || typeof value.ended_at === "string")
  );
}

function isMessage(value: unknown): value is RealtimeMessage {
  return (
    isRecord(value) &&
    (typeof value.id === "number" || typeof value.id === "string") &&
    typeof value.user_id === "number" &&
    typeof value.name === "string" &&
    languageCodes.has(value.language as LanguageCode) &&
    typeof value.text === "string" &&
    typeof value.sent_at === "string"
  );
}

function parseEnvelope(raw: unknown): ServerCallEnvelope | null {
  if (typeof raw !== "string") return null;
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return null;
  }
  if (
    !isRecord(value) ||
    value.version !== 1 ||
    typeof value.type !== "string" ||
    !eventTypes.has(value.type as CallEventType) ||
    (value.request_id !== undefined && typeof value.request_id !== "string") ||
    typeof value.occurred_at !== "string" ||
    !isRecord(value.data)
  ) {
    return null;
  }

  const data = value.data;
  switch (value.type) {
    case CALL_EVENT.JOIN_CALL:
      if (
        !isCall(data.call) ||
        !Array.isArray(data.mute_states) ||
        !data.mute_states.every(
          (state) =>
            isRecord(state) &&
            typeof state.user_id === "number" &&
            typeof state.muted === "boolean",
        )
      )
        return null;
      break;
    case CALL_EVENT.PARTICIPANT_JOINED:
    case CALL_EVENT.LANGUAGE_CHANGED:
      if (!isParticipant(data.participant)) return null;
      break;
    case CALL_EVENT.PARTICIPANT_LEFT:
      if (typeof data.user_id !== "number" || typeof data.reason !== "string")
        return null;
      break;
    case CALL_EVENT.MESSAGE:
      if (!isMessage(data)) return null;
      break;
    case CALL_EVENT.MUTE_STATE:
      if (typeof data.user_id !== "number" || typeof data.muted !== "boolean")
        return null;
      break;
    case CALL_EVENT.CALL_ENDED:
      if (typeof data.ended_by_user_id !== "number") return null;
      break;
    case CALL_EVENT.ERROR:
      if (typeof data.code !== "string" || typeof data.message !== "string")
        return null;
      break;
  }
  return value as ServerCallEnvelope;
}

function socketUrl(code: string): string {
  const configured = import.meta.env.VITE_API_BASE_URL as string | undefined;
  const url = new URL(
    `/api/calls/${encodeURIComponent(code)}/ws`,
    configured || window.location.origin,
  );
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  return url.toString();
}

export class CallSocket {
  private socket: WebSocket | null = null;
  private reconnectTimer: number | undefined;
  private reconnectAttempt = 0;
  private stopped = false;
  private callEnded = false;
  private requestSequence = 0;
  private listeners = new Set<EventListener>();
  private statusListeners = new Set<StatusListener>();
  private binaryListeners = new Set<BinaryListener>();
  private currentStatus: CallSocketStatus = "connecting";

  constructor(
    private readonly code: string,
    private readonly token: string,
  ) {
    this.connect();
  }

  get status(): CallSocketStatus {
    return this.currentStatus;
  }

  subscribe(listener: EventListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  subscribeStatus(listener: StatusListener): () => void {
    this.statusListeners.add(listener);
    listener(this.currentStatus);
    return () => this.statusListeners.delete(listener);
  }

  subscribeBinary(listener: BinaryListener): () => void {
    this.binaryListeners.add(listener);
    return () => this.binaryListeners.delete(listener);
  }

  sendChat(text: string): boolean {
    const clean = text.trim();
    if (!clean || clean.length > 1000)
      return false;
    return this.send({ type: CALL_EVENT.MESSAGE, data: { text: clean } });
  }

  setMuted(muted: boolean): boolean {
    return this.send({ type: CALL_EVENT.MUTE_STATE, data: { muted } });
  }

  sendBinary(payload: ArrayBuffer): boolean {
    if (
      !this.socket ||
      this.socket.readyState !== WebSocket.OPEN ||
      this.currentStatus !== "joined" ||
      this.socket.bufferedAmount > maxAudioBufferedBytes
    ) {
      return false;
    }
    this.socket.send(payload);
    return true;
  }

  stop(): void {
    this.stopped = true;
    window.clearTimeout(this.reconnectTimer);
    this.reconnectTimer = undefined;
    const socket = this.socket;
    this.socket = null;
    if (socket && socket.readyState < WebSocket.CLOSING)
      socket.close(1000, "client stopped");
    this.setStatus("stopped");
  }

  private connect(): void {
    if (this.stopped || this.callEnded) return;
    const socket = new WebSocket(socketUrl(this.code), [
      "symphonia.v1",
      `auth.${this.token}`,
    ]);
    socket.binaryType = "arraybuffer";
    this.socket = socket;

    socket.onopen = () => {
      if (this.socket !== socket) return;
      this.send({ type: CALL_EVENT.JOIN_CALL, data: {} });
    };
    socket.onmessage = (message) => {
      if (this.socket !== socket) return;
      if (message.data instanceof ArrayBuffer) {
        if (this.currentStatus === "joined")
          this.binaryListeners.forEach((listener) => listener(message.data));
        return;
      }
      const envelope = parseEnvelope(message.data);
      if (!envelope) return;
      if (envelope.type === CALL_EVENT.JOIN_CALL) {
        this.reconnectAttempt = 0;
        this.setStatus("joined");
      }
      if (envelope.type === CALL_EVENT.CALL_ENDED) this.callEnded = true;
      this.listeners.forEach((listener) => listener(envelope));
      if (this.callEnded && socket.readyState < WebSocket.CLOSING)
        socket.close(1000, "call ended");
    };
    socket.onclose = (event) => {
      if (this.socket !== socket) return;
      this.socket = null;
      if (this.stopped || this.callEnded) return;
      if (!terminalCloseCodes.has(event.code)) {
        this.setStatus("reconnecting");
        const delay = reconnectDelays[
          Math.min(this.reconnectAttempt, reconnectDelays.length - 1)
        ];
        this.reconnectAttempt += 1;
        this.reconnectTimer = window.setTimeout(() => this.connect(), delay);
      } else {
        this.setStatus("unavailable");
      }
    };
  }

  private send(event: {
    type: ClientCallEnvelope["type"];
    data: Record<string, unknown>;
  }): boolean {
    if (
      !this.socket ||
      this.socket.readyState !== WebSocket.OPEN ||
      (event.type !== CALL_EVENT.JOIN_CALL &&
        this.currentStatus !== "joined")
    ) {
      return false;
    }
    this.requestSequence += 1;
    this.socket.send(
      JSON.stringify({
        version: 1,
        type: event.type,
        request_id: `${Date.now()}-${this.requestSequence}`,
        data: event.data,
      }),
    );
    return true;
  }

  private setStatus(status: CallSocketStatus): void {
    if (this.currentStatus === status) return;
    this.currentStatus = status;
    this.statusListeners.forEach((listener) => listener(status));
  }
}
