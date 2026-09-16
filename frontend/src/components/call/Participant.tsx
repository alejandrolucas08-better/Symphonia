import { Mic, MicOff } from "lucide-react";
import { Avatar } from "../ui/Avatar";
import { AudioWave } from "../ui/AudioWave";
import type { Person } from "../../types/demo";

export function Participant({
  person,
  active,
  muted = false,
  compact = false,
  animated = true,
}: {
  person: Person;
  active: boolean;
  muted?: boolean;
  compact?: boolean;
  animated?: boolean;
}) {
  return (
    <div
      className={`${compact ? "participant-row" : "speaker"} ${active && !muted ? "is-speaking" : ""}`}
    >
      <Avatar initials={person.initials} small={compact} />
      <div className="participant-info">
        <span className="participant-name">
          {person.name}
          {person.id === 1 && <span className="muted"> (você)</span>}
        </span>
        <span className="participant-language">
          {compact ? person.languageCode : person.language}
        </span>
      </div>
      {compact ? (
        <span className="participant-mic">
          {muted ? <MicOff size={16} /> : <Mic size={16} />}
        </span>
      ) : (
        <>
          <AudioWave active={active && !muted && animated} large />
          <span className="speaker-state">
            {muted ? "microfone desativado" : active ? "falando" : "ouvindo"}
          </span>
        </>
      )}
    </div>
  );
}
