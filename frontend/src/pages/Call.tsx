import { useEffect, useRef, useState } from "react";
import { Pause, Play, PhoneOff } from "lucide-react";
import { useNavigate, useParams } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { Participant } from "../components/call/Participant";
import { Transcript } from "../components/call/Transcript";
import { ChatPanel } from "../components/call/ChatPanel";
import { CallControls } from "../components/call/CallControls";
import { Button } from "../components/ui/Button";
import { StatusBadge } from "../components/ui/StatusBadge";
import { initialsFor } from "../components/ui/Avatar";
import { useDemo } from "../contexts/DemoContext";
import { useConversation } from "../hooks/useConversation";
import { languages, participants, phrases } from "../data/mocks";

export function Call() {
  const { id } = useParams();
  const { name, settings } = useDemo();
  const { turn, key, ready, animated, paused, setPaused, next } =
    useConversation();
  const [seconds, setSeconds] = useState(0);
  const dialog = useRef<HTMLDialogElement>(null);
  const navigate = useNavigate();
  useEffect(() => {
    const start = Date.now();
    const interval = window.setInterval(
      () => setSeconds(Math.floor((Date.now() - start) / 1000)),
      1000,
    );
    return () => window.clearInterval(interval);
  }, []);
  const me = {
    ...participants[0],
    name,
    initials: initialsFor(name),
    languageCode: settings.spoken,
    language: languages.find((language) => language.code === settings.spoken)!
      .short,
  };
  const people = [me, participants[1]];
  const speaker = people[turn];
  const source = turn === 0 ? settings.spoken : "EN-US";
  const target = settings.heard;
  const silent = turn === 0 && settings.muted;
  return (
    <div className={`call-page ${paused ? "motion-paused" : ""}`}>
      <AppHeader call />
      <main id="main-content" className="container call-main">
        <div className="call-topline">
          <h1>
            Daily meeting <span className="room-code">/ {id}</span>
          </h1>
          <div className="call-clock">
            <span>
              {String(Math.floor(seconds / 60)).padStart(2, "0")}:
              {String(seconds % 60).padStart(2, "0")}
            </span>
            <button
              className="icon-button"
              aria-label={paused ? "Retomar simulação" : "Pausar simulação"}
              title={paused ? "Retomar simulação" : "Pausar simulação"}
              onClick={() => setPaused(!paused)}
            >
              {paused ? <Play size={15} /> : <Pause size={15} />}
            </button>
          </div>
        </div>
        <div className="call-grid">
          <div className="voice-column">
            <section
              className="voice-stage"
              aria-label="Participante em destaque"
            >
              <div className="stage-label">
                <StatusBadge>TRADUÇÃO ATIVA</StatusBadge>
                <span className="demo-tag">sessão simulada</span>
              </div>
              <Participant
                person={speaker}
                active={!silent}
                muted={silent}
                animated={animated}
              />
            </section>
            <Transcript
              speaker={speaker.name}
              original={
                silent ? "Microfone desativado." : phrases[source][turn]
              }
              translated={
                silent ? "Aguardando sua voz." : phrases[target][turn]
              }
              source={source}
              target={target}
              ready={silent || ready}
              turnKey={key}
              next={next}
            />
          </div>
          <aside className="call-sidebar">
            <section className="participants-panel" aria-label="Participantes">
              <div className="panel-heading">
                <h2>PARTICIPANTES</h2>
                <span className="muted">02 / 02</span>
              </div>
              {people.map((person, index) => (
                <Participant
                  key={person.id}
                  person={person}
                  compact
                  active={index === turn}
                  muted={index === 0 && settings.muted}
                />
              ))}
            </section>
            <ChatPanel />
          </aside>
        </div>
      </main>
      <CallControls onEnd={() => dialog.current?.showModal()} />
      <dialog
        ref={dialog}
        className="end-dialog"
        aria-labelledby="end-title"
        onClick={(event) => {
          if (event.target === event.currentTarget) dialog.current?.close();
        }}
      >
        <PhoneOff size={28} />
        <h2 id="end-title">encerrar a conversa?</h2>
        <p>Você pode começar outra quando quiser.</p>
        <div className="dialog-actions">
          <Button autoFocus onClick={() => dialog.current?.close()}>
            Continuar
          </Button>
          <Button className="danger-button" onClick={() => navigate("/home")}>
            Encerrar chamada
          </Button>
        </div>
      </dialog>
    </div>
  );
}
