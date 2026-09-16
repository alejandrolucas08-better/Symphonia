export type LanguageCode = "PT-BR" | "EN-US" | "ES-ES" | "FR-FR";

export type Person = {
  id: number;
  name: string;
  initials: string;
  language: string;
  languageCode: LanguageCode;
  status: "speaking" | "listening";
};

export type Message = { id: number; sender: string; text: string };
