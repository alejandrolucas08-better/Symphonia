import {
  createContext,
  useContext,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import type { LanguageCode } from "../types/demo";
import { useAuth } from "../hooks/useAuth";

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
  const { user } = useAuth();
  const [name, setName] = useState(user?.name ?? "Alejandro");
  const [settings, setSettings] = useState<Settings>(defaults);

  useEffect(() => {
    if (user) setName(user.name);
  }, [user]);

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