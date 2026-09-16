import { useCallback, useEffect, useRef, useState } from "react";
import {
  AUDIO_SAMPLE_COUNT,
  encodeAudioFrame,
  parseAudioFrame,
} from "../services/audioFrame";
import {
  CallAudioEngine,
  microphoneErrorMessage,
  microphoneErrorState,
  type CallAudioState,
} from "../services/callAudio";
import type { CallSocketStatus } from "../types/realtime";

type Diagnostics = {
  outgoingFrames: number;
  incomingFrames: number;
  liveTracks: number;
};

declare global {
  interface Window {
    __symphoniaAudioDiagnostics?: Diagnostics;
  }
}

export function useCallAudio({
  enabled,
  callCode,
  token,
  listenOnly,
  deviceId,
  muted,
  volume,
  socketStatus,
  sendBinary,
  subscribeBinary,
}: {
  enabled: boolean;
  callCode?: string;
  token?: string | null;
  listenOnly: boolean;
  deviceId: string;
  muted: boolean;
  volume: number;
  socketStatus: CallSocketStatus;
  sendBinary: (payload: ArrayBuffer) => boolean;
  subscribeBinary: (listener: (payload: ArrayBuffer) => void) => () => void;
}) {
  const [state, setState] = useState<CallAudioState>("idle");
  const [error, setError] = useState("");
  const engineRef = useRef<CallAudioEngine | null>(null);
  const statusRef = useRef(socketStatus);
  const mutedRef = useRef(muted);
  const deviceRef = useRef(deviceId);
  const volumeRef = useRef(volume);
  const sendRef = useRef(sendBinary);
  const streamIdRef = useRef(crypto.getRandomValues(new Uint32Array(1))[0] || 1);
  const sequenceRef = useRef(0);
  const timestampRef = useRef(0);
  const discontinuityRef = useRef(true);
  const diagnosticsRef = useRef<Diagnostics | null>(null);
  statusRef.current = socketStatus;
  mutedRef.current = muted;
  deviceRef.current = deviceId;
  volumeRef.current = volume;
  sendRef.current = sendBinary;

  const updateLiveTracks = useCallback(() => {
    if (diagnosticsRef.current)
      diagnosticsRef.current.liveTracks = engineRef.current?.liveTrackCount ?? 0;
  }, []);

  const makeEngine = useCallback(() => {
    const engine = new CallAudioEngine(
      (samples) => {
        const timestamp = timestampRef.current;
        timestampRef.current = (timestamp + AUDIO_SAMPLE_COUNT) >>> 0;
        if (mutedRef.current || statusRef.current !== "joined") {
          discontinuityRef.current = true;
          return;
        }
        const payload = encodeAudioFrame({
          discontinuity: discontinuityRef.current,
          streamId: streamIdRef.current,
          sequence: sequenceRef.current,
          timestamp,
          samples,
        });
        if (sendRef.current(payload)) {
          sequenceRef.current = (sequenceRef.current + 1) >>> 0;
          discontinuityRef.current = false;
          if (diagnosticsRef.current)
            diagnosticsRef.current.outgoingFrames += 1;
        } else {
          discontinuityRef.current = true;
        }
      },
      () => {
        setState("unavailable");
        setError(
          "O microfone foi desconectado. Conecte um dispositivo e tente novamente.",
        );
        updateLiveTracks();
      },
    );
    engineRef.current = engine;
    return engine;
  }, []);

  const acquire = useCallback(async () => {
    setState("requesting");
    setError("");
    const engine = engineRef.current ?? makeEngine();
    try {
      const nextState = await engine.start(
        deviceRef.current,
        mutedRef.current,
        volumeRef.current,
      );
      setState(nextState);
      updateLiveTracks();
      return true;
    } catch (cause) {
      setState(microphoneErrorState(cause));
      setError(microphoneErrorMessage(cause));
      try {
        engine.startPlayback(volumeRef.current);
      } catch {
        // The unavailable state already provides the actionable browser guidance.
      }
      updateLiveTracks();
      return false;
    }
  }, [makeEngine, updateLiveTracks]);

  useEffect(() => {
    if (!new URLSearchParams(window.location.search).has("audioDiagnostics")) return;
    const diagnostics = { outgoingFrames: 0, incomingFrames: 0, liveTracks: 0 };
    diagnosticsRef.current = diagnostics;
    window.__symphoniaAudioDiagnostics = diagnostics;
    return () => {
      diagnostics.liveTracks = 0;
      diagnosticsRef.current = null;
    };
  }, []);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    streamIdRef.current = crypto.getRandomValues(new Uint32Array(1))[0] || 1;
    sequenceRef.current = 0;
    timestampRef.current = 0;
    discontinuityRef.current = true;
    const engine = makeEngine();
    if (listenOnly) {
      try {
        setState(engine.startPlayback(volumeRef.current));
      } catch (cause) {
        setState("unavailable");
        setError(microphoneErrorMessage(cause));
      }
    } else {
      void acquire().then(() => {
        if (cancelled) engine.destroy();
      });
    }
    return () => {
      cancelled = true;
      engine.destroy();
      if (engineRef.current === engine) engineRef.current = null;
      updateLiveTracks();
      setState("idle");
    };
  }, [
    acquire,
    callCode,
    deviceId,
    enabled,
    listenOnly,
    makeEngine,
    token,
    updateLiveTracks,
  ]);

  useEffect(() => {
    engineRef.current?.setMuted(muted);
    updateLiveTracks();
  }, [muted, updateLiveTracks]);

  useEffect(() => engineRef.current?.setVolume(volume), [volume]);

  useEffect(() => {
    if (socketStatus !== "joined") {
      discontinuityRef.current = true;
      engineRef.current?.flushPlayback();
    }
  }, [socketStatus]);

  useEffect(
    () =>
      subscribeBinary((payload) => {
        if (statusRef.current !== "joined") return;
        const frame = parseAudioFrame(payload);
        if (!frame || frame.streamId === streamIdRef.current) return;
        if (diagnosticsRef.current) diagnosticsRef.current.incomingFrames += 1;
        engineRef.current?.play(frame.samples, frame.discontinuity);
      }),
    [subscribeBinary],
  );

  const setMutedImmediately = useCallback(
    async (nextMuted: boolean) => {
      if (nextMuted) {
        mutedRef.current = true;
        engineRef.current?.setMuted(true);
        discontinuityRef.current = true;
        return true;
      }
      if (!engineRef.current?.hasLiveTrack) {
        const acquired = await acquire();
        if (!acquired) return false;
      }
      mutedRef.current = false;
      engineRef.current?.setMuted(false);
      discontinuityRef.current = true;
      updateLiveTracks();
      return true;
    },
    [acquire, updateLiveTracks],
  );

  const activate = useCallback(async () => {
    try {
      const nextState = await engineRef.current?.resume();
      if (nextState) setState(nextState);
    } catch {
      setState("suspended");
    }
  }, []);

  const stop = useCallback(() => {
    engineRef.current?.destroy();
    engineRef.current = null;
    discontinuityRef.current = true;
    setState("idle");
    updateLiveTracks();
  }, [updateLiveTracks]);

  return { state, error, activate, retry: acquire, setMutedImmediately, stop };
}
