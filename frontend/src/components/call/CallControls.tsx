import { useState } from "react";
import {
  Mic,
  MicOff,
  PhoneOff,
  Volume2,
  VolumeX,
  Languages,
  MessageSquare,
  X,
} from "lucide-react";
import { useDemo } from "../../contexts/DemoContext";
import { LanguageSelect } from "../ui/LanguageSelect";
import { StatusBadge } from "../ui/StatusBadge";
import type { LanguageCode } from "../../types/demo";
import { languages } from "../../data/mocks";

export function CallControls({
  onEnd,
  onLanguageChange,
  onMuteChange,
  chatOpen,
  onChatToggle,
}: {
  onEnd: () => void;
  chatOpen: boolean;
  onChatToggle: () => void;
  onMuteChange?: (muted: boolean) => boolean | Promise<boolean>;
  onLanguageChange?: (patch: {
    spoken?: LanguageCode;
    heard?: LanguageCode;
  }) => void;
}) {
  const { settings, updateSettings } = useDemo();
  const [languagesOpen, setLanguagesOpen] = useState(false);
  return (
    <div className="call-controls">
      <div className="container controls-inner">
        <div className="translation-status">
          <StatusBadge>TRADUÇÃO ATIVA</StatusBadge>
          <span className="muted">transcrição demo</span>
        </div>
        <div className="control-group">
          <div className="control-item">
            <button
              className={`icon-button control-button ${settings.muted ? "control-muted" : ""}`}
              aria-label={
                settings.muted ? "Ativar microfone" : "Desativar microfone"
              }
              title={
                settings.muted ? "Ativar microfone" : "Desativar microfone"
              }
              aria-pressed={settings.muted}
              onClick={async () => {
                const muted = !settings.muted;
                if ((await onMuteChange?.(muted)) === false) return;
                updateSettings({ muted });
              }}
            >
              {settings.muted ? <MicOff size={21} /> : <Mic size={21} />}
            </button>
            <span>Eu falo · {settings.spoken.slice(0, 2)}</span>
          </div>
          <div className="control-item volume-control">
            <button
              className="icon-button control-button"
              title={settings.volume === 0 ? "Ativar som" : "Silenciar som"}
              aria-label={
                settings.volume === 0 ? "Ativar som" : "Silenciar som"
              }
              aria-pressed={settings.volume === 0}
              onClick={() =>
                updateSettings({ volume: settings.volume === 0 ? 80 : 0 })
              }
            >
              {settings.volume === 0 ? (
                <VolumeX size={21} />
              ) : (
                <Volume2 size={21} />
              )}
            </button>
            <label className="sr-only" htmlFor="volume">
              Volume
            </label>
            <input
              id="volume"
              aria-valuetext={`${settings.volume}%`}
              type="range"
              min="0"
              max="100"
              value={settings.volume}
              onChange={(event) =>
                updateSettings({ volume: Number(event.target.value) })
              }
            />
            <span>Eu ouço · {settings.heard.slice(0, 2)}</span>
          </div>
          <div className="control-item">
            <button
              className="icon-button control-button"
              title="Configurar idiomas"
              aria-label="Configurar idiomas"
              aria-expanded={languagesOpen}
              aria-controls="call-languages"
              onClick={() => setLanguagesOpen(!languagesOpen)}
            >
              <Languages size={22} />
            </button>
            <span>
              idiomas
            </span>
          </div>
          <div className="control-item">
            <button className="icon-button control-button" title="Abrir chat" aria-label="Abrir chat"
              aria-expanded={chatOpen} aria-controls="call-chat" onClick={onChatToggle}>
              <MessageSquare size={21} />
            </button>
            <span>chat</span>
          </div>
          <div className="control-separator" />
          <div className="control-item">
            <button
              className="icon-button control-button end-call"
              title="Encerrar chamada"
              aria-label="Encerrar chamada"
              onClick={onEnd}
            >
              <PhoneOff size={21} />
            </button>
            <span>encerrar</span>
          </div>
        </div>
        <span className="controls-caption">
          SÓ ÁUDIO.
          <br />
          PRESENÇA INTEIRA.
        </span>
      </div>
      {languagesOpen && (
        <section
          id="call-languages"
          className="language-popover"
          aria-label="Idiomas da chamada"
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              setLanguagesOpen(false);
              document
                .querySelector<HTMLButtonElement>(
                  '[aria-controls="call-languages"]',
                )
                ?.focus();
            }
          }}
        >
          <div className="panel-heading">
            <h2>IDIOMAS</h2>
            <button
              className="icon-button"
              aria-label="Fechar idiomas"
              title="Fechar idiomas"
              onClick={() => setLanguagesOpen(false)}
            >
              <X size={18} />
            </button>
          </div>
          <LanguageSelect
            label="eu falo"
            value={settings.spoken}
            onChange={(spoken) => {
              updateSettings({ spoken });
              onLanguageChange?.({ spoken });
            }}
          />
          <LanguageSelect
            label="Ouço a outra pessoa em"
            value={settings.heard}
            onChange={(heard) => {
              updateSettings({ heard });
              onLanguageChange?.({ heard });
            }}
          />
          <p className="muted">Você fala em {languages.find((language) => language.code === settings.spoken)?.name ?? settings.spoken} e ouve a outra pessoa em {languages.find((language) => language.code === settings.heard)?.name ?? settings.heard}. A tradução é automática quando necessária.</p>
        </section>
      )}
    </div>
  );
}
