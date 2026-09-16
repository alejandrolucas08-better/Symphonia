import type { LanguageCode, Message, Person } from "../types/demo";

export const languages: { code: LanguageCode; name: string; short: string }[] =
  [
    { code: "PT-BR", name: "Português · Brasil", short: "Português" },
    { code: "EN-US", name: "English", short: "English" },
    { code: "ES-ES", name: "Español", short: "Español" },
    { code: "FR-FR", name: "Français", short: "Français" },
  ];

export const participants: Person[] = [
  {
    id: 1,
    name: "Alejandro",
    initials: "AL",
    language: "Português",
    languageCode: "PT-BR",
    status: "speaking",
  },
  {
    id: 2,
    name: "Mateus",
    initials: "MA",
    language: "English",
    languageCode: "EN-US",
    status: "listening",
  },
];

export const transcripts = [
  {
    speaker: "Alejandro",
    original: "Como está o projeto?",
    translated: "How is the project going?",
  },
  {
    speaker: "Mateus",
    original: "Everything is going well.",
    translated: "Está tudo indo bem.",
  },
];

export const phrases: Record<LanguageCode, string[]> = {
  "PT-BR": [transcripts[0].original, transcripts[1].translated],
  "EN-US": [transcripts[0].translated, transcripts[1].original],
  "ES-ES": ["¿Cómo va el proyecto?", "Todo va bien."],
  "FR-FR": ["Comment avance le projet ?", "Tout se passe bien."],
};

export const messages: Message[] = [
  { id: 1, sender: "Alejandro", text: "Bom dia" },
  { id: 2, sender: "Mateus", text: "Good morning" },
];
