import { createContext, useContext, useState, type ReactNode } from "react";
import type { LanguageCode } from "../types/demo";

type Settings = {
  spoken: LanguageCode;
  heard: LanguageCode;
  muted: boolean;
  volume: number;
};
type DemoState = {
  name: string;
  setName: (name: string) => void;
  settings: Settings;
  updateSettings: (patch: Partial<Settings>) => void;
  reset: () => void;
};
const defaults: Settings = {
  spoken: "PT-BR",
  heard: "EN-US",
  muted: false,
  volume: 80,
};
const DemoContext = createContext<DemoState | null>(null);

export function DemoProvider({ children }: { children: ReactNode }) {
  const [name, setName] = useState("Alejandro");
  const [settings, setSettings] = useState<Settings>(defaults);
  return (
    <DemoContext.Provider
      value={{
        name,
        setName,
        settings,
        updateSettings: (patch) =>
          setSettings((current) => ({ ...current, ...patch })),
        reset: () => {
          setName("Alejandro");
          setSettings(defaults);
        },
      }}
    >
      {children}
    </DemoContext.Provider>
  );
}

export function useDemo() {
  const value = useContext(DemoContext);
  if (!value) throw new Error("useDemo requires DemoProvider");
  return value;
}
