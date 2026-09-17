import type { Call, CallParticipant } from "./call";
import type { LanguageCode } from "./demo";

export const CALL_EVENT = {
  JOIN_CALL: "join_call",
  PARTICIPANT_JOINED: "participant_joined",
  PARTICIPANT_LEFT: "participant_left",
  MESSAGE: "message",
  MUTE_STATE: "mute_state",
  LANGUAGE_CHANGED: "language_changed",
  CALL_ENDED: "call_ended",
  INPUT_TRANSCRIPTION: "input_transcription",
  OUTPUT_TRANSCRIPTION: "output_transcription",
  TRANSLATED_AUDIO: "translated_audio",
  TRANSLATION_INTERRUPTED: "translation_interrupted",
  ERROR: "error",
} as const;

export type CallEventType = (typeof CALL_EVENT)[keyof typeof CALL_EVENT];

export type RealtimeMessage = {
  id: number | string;
  user_id: number;
  name: string;
  language: LanguageCode;
  text: string;
  sent_at: string;
};

export type MuteState = { user_id: number; muted: boolean };

export type ClientCallEvent =
  | { type: typeof CALL_EVENT.JOIN_CALL; data: Record<string, never> }
  | { type: typeof CALL_EVENT.MESSAGE; data: { text: string } }
  | { type: typeof CALL_EVENT.MUTE_STATE; data: { muted: boolean } };

export type ClientCallEnvelope = ClientCallEvent extends infer Event
  ? Event extends ClientCallEvent
    ? Event & { version: 1; request_id: string }
    : never
  : never;

type ServerEventData =
  | {
      type: typeof CALL_EVENT.JOIN_CALL;
      data: { call: Call; mute_states: MuteState[] };
    }
  | {
      type: typeof CALL_EVENT.PARTICIPANT_JOINED;
      data: { participant: CallParticipant };
    }
  | {
      type: typeof CALL_EVENT.PARTICIPANT_LEFT;
      data: { user_id: number; reason: string };
    }
  | { type: typeof CALL_EVENT.MESSAGE; data: RealtimeMessage }
  | { type: typeof CALL_EVENT.MUTE_STATE; data: MuteState }
  | {
      type: typeof CALL_EVENT.LANGUAGE_CHANGED;
      data: { participant: CallParticipant };
    }
  | {
      type: typeof CALL_EVENT.CALL_ENDED;
      data: { ended_by_user_id: number };
    }
  | {
      type:
        | typeof CALL_EVENT.INPUT_TRANSCRIPTION
        | typeof CALL_EVENT.OUTPUT_TRANSCRIPTION;
      data: { user_id: number; text: string; language: LanguageCode };
    }
  | {
      type: typeof CALL_EVENT.TRANSLATED_AUDIO;
      data: {
        user_id: number;
        mime_type: "audio/pcm;rate=24000";
        data: string;
      };
    }
  | {
      type: typeof CALL_EVENT.TRANSLATION_INTERRUPTED;
      data: { user_id: number };
    }
  | {
      type: typeof CALL_EVENT.ERROR;
      data: { code: string; message: string };
    };

export type ServerCallEnvelope = ServerEventData extends infer Event
  ? Event extends ServerEventData
    ? Event & { version: 1; request_id?: string; occurred_at: string }
    : never
  : never;

export type CallSocketStatus =
  | "connecting"
  | "joined"
  | "reconnecting"
  | "unavailable"
  | "stopped";
