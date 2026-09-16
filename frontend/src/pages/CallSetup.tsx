import { ArrowLeft, ArrowUpRight, Mic, MicOff, RefreshCw } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { AudioWave } from "../components/ui/AudioWave";
import { Avatar, initialsFor } from "../components/ui/Avatar";
import { LanguageSelect } from "../components/ui/LanguageSelect";
import { StatusBadge } from "../components/ui/StatusBadge";
import { useDemo } from "../contexts/DemoContext";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";
import { microphoneErrorMessage, requestMicrophone } from "../services/callAudio";
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
  const [microphones, setMicrophones] = useState<MediaDeviceInfo[]>([]);
  const [microphoneStatus, setMicrophoneStatus] = useState<
    "idle" | "requesting" | "ready" | "error"
  >("idle");
  const [microphoneError, setMicrophoneError] = useState("");
  const setupStream = useRef<MediaStream | null>(null);

  const stopSetupStream = () => {
    setupStream.current?.getTracks().forEach((track) => track.stop());
    setupStream.current = null;
  };

  const listMicrophones = async () => {
    if (!navigator.mediaDevices?.enumerateDevices) {
      setMicrophoneStatus("error");
      setMicrophoneError("Este navegador não oferece acesso a dispositivos de áudio.");
      return;
    }
    try {
      const devices = (await navigator.mediaDevices.enumerateDevices()).filter(
        (device) => device.kind === "audioinput",
      );
      setMicrophones(devices);
      if (
        devices.length > 0 &&
        !devices.some((device) => device.deviceId === settings.microphoneDeviceId)
      ) {
        updateSettings({ microphoneDeviceId: devices[0].deviceId });
      }
      if (devices.length === 0) {
        setMicrophoneStatus("error");
        setMicrophoneError("Nenhum microfone foi encontrado. Conecte um dispositivo e atualize a lista.");
      }
    } catch (cause) {
      setMicrophoneStatus("error");
      setMicrophoneError(microphoneErrorMessage(cause));
    }
  };

  useEffect(() => {
    void listMicrophones();
    return stopSetupStream;
  }, []);

  const requestPermissionAndRefresh = async () => {
    if (microphoneStatus === "requesting") return;
    stopSetupStream();
    setMicrophoneStatus("requesting");
    setMicrophoneError("");
    try {
      setupStream.current = await requestMicrophone("");
      await listMicrophones();
      setMicrophoneStatus("ready");
      updateSettings({ listenOnly: false });
    } catch (cause) {
      setMicrophoneStatus("error");
      setMicrophoneError(microphoneErrorMessage(cause));
    } finally {
      stopSetupStream();
    }
  };

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

  const enterCall = async (listenOnly = false) => {
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
      if (!listenOnly) {
        setMicrophoneStatus("requesting");
        setMicrophoneError("");
        try {
          setupStream.current = await requestMicrophone(
            settings.microphoneDeviceId,
          );
          setMicrophoneStatus("ready");
          updateSettings({ listenOnly: false });
        } catch (cause) {
          setMicrophoneStatus("error");
          setMicrophoneError(microphoneErrorMessage(cause));
          setPending(false);
          return;
        } finally {
          stopSetupStream();
        }
      } else {
        updateSettings({ listenOnly: true, muted: true });
      }
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
          <span className="demo-tag">áudio do navegador</span>
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
              aria-label="Configuração do microfone"
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
                <select
                  id="microphone"
                  value={settings.microphoneDeviceId}
                  onChange={(event) =>
                    updateSettings({
                      microphoneDeviceId: event.target.value,
                      listenOnly: false,
                    })
                  }
                  disabled={microphones.length === 0}
                >
                  {microphones.length === 0 && (
                    <option value="">Nenhum microfone disponível</option>
                  )}
                  {microphones.map((device, index) => (
                    <option key={device.deviceId || `microphone-${index}`} value={device.deviceId}>
                      {device.label || `Microfone ${index + 1}`}
                    </option>
                  ))}
                </select>
                <button
                  className="subtle-link device-refresh"
                  type="button"
                  disabled={microphoneStatus === "requesting"}
                  onClick={() => void requestPermissionAndRefresh()}
                >
                  <RefreshCw size={15} />
                  {microphoneStatus === "requesting"
                    ? "Solicitando acesso..."
                    : "Permitir e atualizar microfones"}
                </button>
                {microphoneStatus === "ready" && (
                  <p className="audio-state" role="status">Microfone disponível.</p>
                )}
                {microphoneError && (
                  <div className="microphone-error">
                    <p className="error-message" role="alert">{microphoneError}</p>
                    <button
                      className="subtle-link"
                      type="button"
                      disabled={pending}
                      onClick={() => void enterCall(true)}
                    >
                      Entrar somente para ouvir
                    </button>
                  </div>
                )}
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
                onClick={() => void enterCall(settings.listenOnly)}
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
