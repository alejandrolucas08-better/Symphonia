import { useState } from "react";
import { ArrowUpRight, AudioLines, Plus } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { ActionButton } from "../components/ui/Button";
import { useDemo } from "../contexts/DemoContext";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";

export function Home() {
  const { name, settings } = useDemo();
  const { token, user } = useAuth();
  const navigate = useNavigate();
  const [meetingName, setMeetingName] = useState("");
  const [code, setCode] = useState("");
  const [createError, setCreateError] = useState("");
  const [joinError, setJoinError] = useState("");
  const [createPending, setCreatePending] = useState(false);
  const [joinPending, setJoinPending] = useState(false);
  const hour = new Date().getHours();
  const greeting =
    hour < 12 ? "bom dia" : hour < 18 ? "boa tarde" : "boa noite";
  return (
    <>
      <AppHeader />
      <main id="main-content" className="container home-page">
        <div className="home-heading">
          <div>
            <p className="eyebrow">SEU PONTO DE ENCONTRO</p>
            <h1>
              {greeting},<br />
              <strong>{name.toUpperCase()}.</strong>
            </h1>
            <p className="muted">o que vamos fazer?</p>
          </div>
          <AudioLines className="home-mark" size={80} strokeWidth={0.8} />
        </div>
        <div className="home-actions">
          <section className="home-action">
            <div className="action-number">
              <span>01</span>
              <Plus size={24} strokeWidth={1} />
            </div>
            <h2>
              CRIAR
              <br />
              CHAMADA
            </h2>
            <p>
              uma nova conversa.
              <br />
              uma nova conexão.
            </p>
            <form
              className="create-call-form"
              onSubmit={async (event) => {
                event.preventDefault();
                if (!token || createPending) return;
                const cleanName = meetingName.trim() || "Meeting";
                setCreatePending(true);
                setCreateError("");
                try {
                  const { data } = await api.createCall(token, {
                    name: cleanName,
                    spoken_language: settings.spoken,
                    heard_language: settings.heard,
                  });
                  navigate(`/call/${encodeURIComponent(data.code)}/setup`);
                } catch (err) {
                  setCreateError(userFacingError(err));
                  setCreatePending(false);
                }
              }}
            >
              <div className="meeting-name-field">
                <label htmlFor="meeting-name">nome da reunião</label>
                <input
                  id="meeting-name"
                  value={meetingName}
                  placeholder="Meeting"
                  maxLength={80}
                  aria-invalid={!!createError}
                  aria-describedby={createError ? "create-error" : undefined}
                  onChange={(event) => {
                    setMeetingName(event.target.value);
                    setCreateError("");
                  }}
                />
              </div>
              <ActionButton
                type="submit"
                disabled={createPending}
                aria-busy={createPending}
              >
                {createPending ? "Criando chamada..." : "Criar chamada"}
              </ActionButton>
              {createError && (
                <p id="create-error" role="alert" className="error-message">
                  {createError}
                </p>
              )}
            </form>
          </section>
          <section className="home-action">
            <div className="action-number">
              <span>02</span>
              <ArrowUpRight size={24} strokeWidth={1} />
            </div>
            <h2>
              ENTRAR
              <br />
              EM CHAMADA
            </h2>
            <p>
              já tem um convite?
              <br />a conversa espera por você.
            </p>
            <form
              onSubmit={async (event) => {
                event.preventDefault();
                const clean = code.trim().toLowerCase();
                if (!/^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(clean)) {
                  setJoinError("Use letras, números e hífens entre palavras.");
                  return;
                }
                if (!token || joinPending) return;
                setJoinPending(true);
                setJoinError("");
                try {
                  const { data } = await api.getCall(token, clean);
                  if (data.status === "ended") {
                    setJoinError("Esta chamada já foi encerrada.");
                    return;
                  }
                  const alreadyJoined = data.participants.some(
                    (participant) => participant.user_id === user?.id,
                  );
                  if (data.participants.length >= 2 && !alreadyJoined) {
                    setJoinError("A chamada já está cheia.");
                    return;
                  }
                  navigate(
                    `/call/${encodeURIComponent(data.code)}/setup`,
                  );
                } catch (err) {
                  setJoinError(userFacingError(err));
                } finally {
                  setJoinPending(false);
                }
              }}
            >
              <div className="join-field">
                <label className="sr-only" htmlFor="room-code">
                  Código da chamada
                </label>
                <input
                  id="room-code"
                  placeholder="inserir código"
                  value={code}
                  onChange={(event) => {
                    setCode(event.target.value);
                    setJoinError("");
                  }}
                  maxLength={40}
                  required
                  aria-invalid={!!joinError}
                  aria-describedby={joinError ? "code-error" : undefined}
                />
                <button
                  className="icon-button join-button"
                  aria-label="Entrar com código"
                  title="Entrar com código"
                  disabled={joinPending}
                  aria-busy={joinPending}
                >
                  <ArrowUpRight size={22} />
                </button>
              </div>
              {joinError && (
                <p id="code-error" role="alert" className="error-message">
                  {joinError}
                </p>
              )}
            </form>
          </section>
        </div>
        <footer className="home-foot">
          <span>TRADUÇÃO EM TEMPO REAL</span>
          <span>ATÉ 2 PESSOAS / SÓ ÁUDIO</span>
        </footer>
      </main>
    </>
  );
}
