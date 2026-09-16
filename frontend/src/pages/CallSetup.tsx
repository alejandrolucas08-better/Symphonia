import { ArrowLeft, ArrowUpRight, Mic, MicOff } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { AudioWave } from "../components/ui/AudioWave";
import { Avatar, initialsFor } from "../components/ui/Avatar";
import { LanguageSelect } from "../components/ui/LanguageSelect";
import { StatusBadge } from "../components/ui/StatusBadge";
import { useDemo } from "../contexts/DemoContext";

export function CallSetup() {
  const { id } = useParams();
  const { name, settings, updateSettings } = useDemo();
  return (
    <>
      <AppHeader />
      <main id="main-content" className="container setup-page">
        <Link to="/home" className="subtle-link">
          <ArrowLeft size={16} /> voltar
        </Link>
        <div className="setup-heading">
          <div>
            <p className="eyebrow">
              PRÉ-CHAMADA / <span className="room-code">{id}</span>
            </p>
            <h1>
              vamos nos
              <br />
              <strong>sintonizar.</strong>
            </h1>
          </div>
          <span className="demo-tag">simulação de áudio</span>
        </div>
        <div className="setup-grid">
          <section
            className="microphone-preview"
            aria-label="Prévia do microfone simulado"
          >
            <Avatar initials={initialsFor(name)} />
            <p className="preview-name">{name}</p>
            <AudioWave active={!settings.muted} large />
            <StatusBadge active={!settings.muted}>
              {settings.muted ? "microfone desativado" : "microfone ativo"}
            </StatusBadge>
            <button
              className="icon-button microphone-toggle"
              aria-label={
                settings.muted ? "Ativar microfone" : "Desativar microfone"
              }
              title={
                settings.muted ? "Ativar microfone" : "Desativar microfone"
              }
              aria-pressed={settings.muted}
              onClick={() => updateSettings({ muted: !settings.muted })}
            >
              {settings.muted ? <MicOff size={22} /> : <Mic size={22} />}
            </button>
          </section>
          <section className="setup-fields" aria-label="Configurações de áudio">
            <div className="field">
              <label htmlFor="microphone">microfone</label>
              <select id="microphone" defaultValue="default">
                <option value="default">Microfone padrão (simulado)</option>
              </select>
            </div>
            <LanguageSelect
              label="eu falo"
              value={settings.spoken}
              onChange={(spoken) => updateSettings({ spoken })}
            />
            <LanguageSelect
              label="quero ouvir"
              value={settings.heard}
              onChange={(heard) => updateSettings({ heard })}
            />
            <Link
              className="action-link form-submit"
              to={`/call/${encodeURIComponent(id ?? "demo-room")}`}
            >
              Entrar na chamada{" "}
              <span className="action-arrow">
                <ArrowUpRight size={22} />
              </span>
            </Link>
          </section>
        </div>
      </main>
    </>
  );
}
