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
import { useAuth } from "../hooks/useAuth";
import { useCallSocket } from "../hooks/useCallSocket";
import { useCallAudio } from "../hooks/useCallAudio";
import { languages, phrases } from "../data/mocks";
import { api, userFacingError } from "../services/api";
import type { Call as CallData } from "../types/call";
import type { LanguageCode, Person } from "../types/demo";
import {
  CALL_EVENT,
  type RealtimeMessage,
  type ServerCallEnvelope,
} from "../types/realtime";

export function Call() {
  const { id } = useParams();
  const { settings, updateSettings } = useDemo();
  const { token, user } = useAuth();
  const { turn, key, ready, animated, paused, setPaused, next } =
    useConversation();
  const [call, setCall] = useState<CallData | null>(null);
  const [loading, setLoading] = useState(true);
  const [requestError, setRequestError] = useState("");
  const [endPending, setEndPending] = useState(false);
  const [seconds, setSeconds] = useState(0);
  const [messages, setMessages] = useState<RealtimeMessage[]>([]);
  const [muteStates, setMuteStates] = useState<Map<number, boolean>>(new Map());
  const dialog = useRef<HTMLDialogElement>(null);
  const refreshCall = useRef<() => void>(() => undefined);
  const navigate = useNavigate();

  useEffect(() => {
    const start = Date.now();
    const interval = window.setInterval(
      () => setSeconds(Math.floor((Date.now() - start) / 1000)),
      1000,
    );
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    if (!token || !id) return;
    let active = true;
    const refresh = async (initial = false) => {
      try {
        const { data } = await api.getCall(token, id);
        if (!active) return;
        if (data.status === "ended") {
          navigate("/home", { replace: true });
          return;
        }
        if (
          !data.participants.some(
            (participant) => participant.user_id === user?.id,
          )
        ) {
          navigate(`/call/${encodeURIComponent(data.code)}/setup`, {
            replace: true,
          });
          return;
        }
        setCall(data);
        setRequestError("");
      } catch (err) {
        if (active) setRequestError(userFacingError(err));
      } finally {
        if (active && initial) setLoading(false);
      }
    };
    refreshCall.current = () => void refresh();
    void refresh(true);
    const interval = window.setInterval(() => void refresh(), 3000);
    return () => {
      active = false;
      window.clearInterval(interval);
    };
  }, [id, navigate, token, user?.id]);

  useEffect(() => {
    setMessages([]);
    setMuteStates(new Map());
  }, [id]);

  const handleSocketEvent = (event: ServerCallEnvelope) => {
    switch (event.type) {
      case CALL_EVENT.JOIN_CALL: {
        if (event.data.call.status === "ended") {
          navigate("/home", { replace: true });
          return;
        }
        setCall(event.data.call);
        setMuteStates(
          new Map(
            event.data.mute_states.map(({ user_id, muted }) => [
              user_id,
              muted,
            ]),
          ),
        );
        socket.setMuted(settings.muted);
        break;
      }
      case CALL_EVENT.PARTICIPANT_JOINED:
      case CALL_EVENT.PARTICIPANT_LEFT:
      case CALL_EVENT.LANGUAGE_CHANGED:
        refreshCall.current();
        break;
      case CALL_EVENT.MESSAGE:
        setMessages((current) =>
          current.some((message) => message.id === event.data.id)
            ? current
            : [...current, event.data],
        );
        break;
      case CALL_EVENT.MUTE_STATE:
        setMuteStates((current) => {
          const next = new Map(current);
          next.set(event.data.user_id, event.data.muted);
          return next;
        });
        if (event.data.user_id === user?.id)
          updateSettings({ muted: event.data.muted });
        break;
      case CALL_EVENT.CALL_ENDED:
        navigate("/home", { replace: true });
        break;
      case CALL_EVENT.ERROR:
        setRequestError(event.data.message);
        break;
    }
  };

  const socket = useCallSocket({
    code: call?.code,
    token,
    enabled:
      call?.status !== "ended" &&
      Boolean(call?.participants.some((item) => item.user_id === user?.id)),
    onEvent: handleSocketEvent,
  });
  const audio = useCallAudio({
    enabled:
      call?.status !== "ended" &&
      Boolean(call?.participants.some((item) => item.user_id === user?.id)),
    callCode: call?.code,
    token,
    listenOnly: settings.listenOnly,
    deviceId: settings.microphoneDeviceId,
    muted: settings.muted,
    volume: settings.volume,
    socketStatus: socket.status,
    sendBinary: socket.sendBinary,
    subscribeBinary: socket.subscribeBinary,
  });

  const updateLanguages = async (patch: {
    spoken?: LanguageCode;
    heard?: LanguageCode;
  }) => {
    if (!token || !call) return;
    try {
      const { data } = await api.updateCallLanguage(token, call.code, {
        spoken_language: patch.spoken ?? settings.spoken,
        heard_language: patch.heard ?? settings.heard,
      });
      setCall(data);
      setRequestError("");
    } catch (err) {
      setRequestError(userFacingError(err));
    }
  };

  const exitCall = async () => {
    if (!token || !call || endPending) return;
    setEndPending(true);
    setRequestError("");
    try {
      if (call.host_user_id === user?.id) {
        await api.endCall(token, call.code);
      } else {
        await api.leaveCall(token, call.code);
      }
      audio.stop();
      socket.stop();
      dialog.current?.close();
      navigate("/home");
    } catch (err) {
      setRequestError(userFacingError(err));
      setEndPending(false);
    }
  };

  if (loading || !call) {
    return (
      <div className="call-page">
        <AppHeader call />
        <main id="main-content" className="container call-main">
          <h1>
            Daily meeting <span className="room-code">/ {id}</span>
          </h1>
          {loading ? (
            <p role="status">Carregando chamada...</p>
          ) : (
            <p role="alert" className="error-message">
              {requestError}
            </p>
          )}
        </main>
      </div>
    );
  }

  const people: Person[] = call.participants.map((participant) => ({
    id: participant.user_id,
    name: participant.name,
    initials: initialsFor(participant.name),
    languageCode: participant.spoken_language,
    language:
      languages.find(
        (language) => language.code === participant.spoken_language,
      )?.short ?? participant.spoken_language,
    status: "listening",
  }));
  const currentParticipant = call.participants.find(
    (participant) => participant.user_id === user?.id,
  );
  const featured = people.length === 1 ? [people[0], people[0]] : people;
  const speaker = featured[turn % featured.length];
  const source = speaker?.languageCode ?? settings.spoken;
  const target = currentParticipant?.heard_language ?? settings.heard;
  const silent = speaker
    ? (muteStates.get(speaker.id) ??
      (speaker.id === user?.id ? settings.muted : false))
    : false;
  const isHost = call.host_user_id === user?.id;

  return (
    <div className={`call-page ${paused ? "motion-paused" : ""}`}>
      <AppHeader
        call
        participantCount={people.length}
        realtimeStatus={socket.status}
      />
      <main id="main-content" className="container call-main">
        <div className="call-topline">
          <h1>
            Daily meeting <span className="room-code">/ {call.code}</span>
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
        {requestError && (
          <p role="alert" className="error-message">
            {requestError}
          </p>
        )}
        <div className="call-audio-status" aria-live="polite">
          <span>
            {audio.state === "requesting" && "Solicitando microfone..."}
            {audio.state === "denied" && "Microfone sem permissão."}
            {audio.state === "unavailable" && "Microfone indisponível."}
            {audio.state === "active" &&
              (settings.listenOnly ? "Áudio de escuta ativo." : "Áudio ativo.")}
            {audio.state === "suspended" && "Áudio aguardando ativação."}
          </span>
          {audio.state === "suspended" && (
            <button className="subtle-link" type="button" onClick={() => void audio.activate()}>
              Ativar áudio
            </button>
          )}
          {(audio.state === "denied" || audio.state === "unavailable") && (
            <button className="subtle-link" type="button" onClick={() => void audio.retry()}>
              Tentar microfone novamente
            </button>
          )}
        </div>
        {audio.error && (audio.state === "denied" || audio.state === "unavailable") && (
          <p className="error-message" role="alert">{audio.error}</p>
        )}
        {call.status === "ended" && (
          <p role="status" className="error-message">
            Esta chamada já foi encerrada.
          </p>
        )}
        {speaker && (
          <div className="call-grid">
            <div className="voice-column">
              <section
                className="voice-stage"
                aria-label="Participante em destaque"
              >
                <div className="stage-label">
                  <StatusBadge>TRADUÇÃO ATIVA</StatusBadge>
                  <span className="demo-tag">transcrição demonstrativa</span>
                </div>
                <Participant
                  person={speaker}
                  currentUserId={user?.id}
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
                  <span className="muted">
                    {String(people.length).padStart(2, "0")} / 02
                  </span>
                </div>
                {people.map((person) => (
                  <Participant
                    key={person.id}
                    person={person}
                    currentUserId={user?.id}
                    compact
                    active={person.id === speaker.id}
                    muted={
                      muteStates.get(person.id) ??
                      (person.id === user?.id && settings.muted)
                    }
                  />
                ))}
              </section>
              <ChatPanel
                messages={messages}
                currentUserId={user?.id}
                connectionStatus={socket.status}
                onSend={socket.sendChat}
              />
            </aside>
          </div>
        )}
      </main>
      <CallControls
        onEnd={() => dialog.current?.showModal()}
        onLanguageChange={(patch) => void updateLanguages(patch)}
        onMuteChange={async (muted) => {
          const changed = await audio.setMutedImmediately(muted);
          if (!changed) return false;
          socket.setMuted(muted);
          return true;
        }}
      />
      <dialog
        ref={dialog}
        className="end-dialog"
        aria-labelledby="end-title"
        onClick={(event) => {
          if (event.target === event.currentTarget && !endPending)
            dialog.current?.close();
        }}
      >
        <PhoneOff size={28} />
        <h2 id="end-title">
          {isHost ? "encerrar a conversa?" : "sair da conversa?"}
        </h2>
        <p>Você pode começar outra quando quiser.</p>
        {requestError && (
          <p role="alert" className="error-message">
            {requestError}
          </p>
        )}
        <div className="dialog-actions">
          <Button
            autoFocus
            disabled={endPending}
            onClick={() => dialog.current?.close()}
          >
            Continuar
          </Button>
          <Button
            className="danger-button"
            disabled={endPending}
            aria-busy={endPending}
            onClick={exitCall}
          >
            {endPending
              ? isHost
                ? "Encerrando..."
                : "Saindo..."
              : isHost
                ? "Encerrar chamada"
                : "Sair da chamada"}
          </Button>
        </div>
      </dialog>
    </div>
  );
}
