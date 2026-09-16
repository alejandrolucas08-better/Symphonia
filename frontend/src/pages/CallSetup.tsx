import { ArrowLeft, ArrowUpRight, Mic, MicOff } from "lucide-react";
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { AudioWave } from "../components/ui/AudioWave";
import { Avatar, initialsFor } from "../components/ui/Avatar";
import { LanguageSelect } from "../components/ui/LanguageSelect";
import { StatusBadge } from "../components/ui/StatusBadge";
import { useDemo } from "../contexts/DemoContext";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";
import type { Call } from "../types/call";

export function CallSetup() {
  const { id } = useParams();
  const { name, settings, updateSettings } = useDemo();
  const { token, user } = useAuth();
  const navigate = useNavigate();
  const [call, setCall] = useState<Call | null>(null);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token || !id) return;
    let active = true;
    setLoading(true);
    setError("");
    api
      .getCall(token, id)
      .then(({ data }) => {
        if (!active) return;
        setCall(data);
        if (data.status === "ended") {
          setError("Esta chamada já foi encerrada.");
        }
        const participant = data.participants.find(
          (item) => item.user_id === user?.id,
        );
        if (data.participants.length >= 2 && !participant) {
          setError("A chamada já está cheia.");
        }
        if (participant) {
          updateSettings({
            spoken: participant.spoken_language,
            heard: participant.heard_language,
          });
        }
      })
      .catch((err) => {
        if (active) setError(userFacingError(err));
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [id, token, user?.id]);

  const enterCall = async () => {
    if (!token || !call || pending) return;
    if (call.status === "ended") {
      setError("Esta chamada já foi encerrada.");
      return;
    }
    const alreadyJoined = call.participants.some(
      (participant) => participant.user_id === user?.id,
    );
    if (call.participants.length >= 2 && !alreadyJoined) {
      setError("A chamada já está cheia.");
      return;
    }
    setPending(true);
    setError("");
    try {
      const { data } = await api.joinCall(token, call.code, {
        spoken_language: settings.spoken,
        heard_language: settings.heard,
      });
      navigate(`/call/${encodeURIComponent(data.code)}`);
    } catch (err) {
      setError(userFacingError(err));
      setPending(false);
    }
  };
  const alreadyJoined = call?.participants.some(
    (participant) => participant.user_id === user?.id,
  );
  const unavailable =
    call?.status === "ended" ||
    (!!call && call.participants.length >= 2 && !alreadyJoined);

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
              PRÉ-CHAMADA / <span className="room-code">{call?.code ?? id}</span>
            </p>
            <h1>
              vamos nos
              <br />
              <strong>sintonizar.</strong>
            </h1>
          </div>
          <span className="demo-tag">simulação de áudio</span>
        </div>
        {loading && <p role="status">Carregando chamada...</p>}
        {error && (
          <p className="error-message" role="alert">
            {error}
          </p>
        )}
        {!loading && call && !unavailable && (
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
            <section
              className="setup-fields"
              aria-label="Configurações de áudio"
            >
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
              <button
                className="action-link form-submit"
                type="button"
                disabled={pending}
                aria-busy={pending}
                onClick={enterCall}
              >
                {pending ? "Entrando..." : "Entrar na chamada"}{" "}
                <span className="action-arrow">
                  <ArrowUpRight size={22} />
                </span>
              </button>
            </section>
          </div>
        )}
      </main>
    </>
  );
}
