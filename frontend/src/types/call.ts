import type { LanguageCode } from "./demo";

export type CallParticipant = {
  user_id: number;
  name: string;
  spoken_language: LanguageCode;
  heard_language: LanguageCode;
  joined_at: string;
};

export type Call = {
  id: number;
  code: string;
  host_user_id: number;
  status: "waiting" | "active" | "ended";
  participants: CallParticipant[];
  created_at: string;
  updated_at: string;
  ended_at: string | null;
};

export type CallLanguagePayload = {
  spoken_language: LanguageCode;
  heard_language: LanguageCode;
};
