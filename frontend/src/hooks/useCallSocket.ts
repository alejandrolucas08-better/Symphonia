import { useCallback, useEffect, useRef, useState } from "react";
import { CallSocket } from "../services/callSocket";
import type {
  CallSocketStatus,
  ServerCallEnvelope,
} from "../types/realtime";

export function useCallSocket({
  code,
  token,
  enabled,
  onEvent,
}: {
  code?: string;
  token?: string | null;
  enabled: boolean;
  onEvent: (event: ServerCallEnvelope) => void;
}) {
  const socketRef = useRef<CallSocket | null>(null);
  const eventRef = useRef(onEvent);
  const binaryListenersRef = useRef(
    new Set<(payload: ArrayBuffer) => void>(),
  );
  const eventListenersRef = useRef(
    new Set<(event: ServerCallEnvelope) => void>(),
  );
  const [status, setStatus] = useState<CallSocketStatus>("stopped");
  eventRef.current = onEvent;

  useEffect(() => {
    if (!enabled || !code || !token) {
      setStatus("stopped");
      return;
    }
    const socket = new CallSocket(code, token);
    socketRef.current = socket;
    const unsubscribe = socket.subscribe((event) => {
      eventRef.current(event);
      eventListenersRef.current.forEach((listener) => listener(event));
    });
    const unsubscribeStatus = socket.subscribeStatus(setStatus);
    const unsubscribeBinary = socket.subscribeBinary((payload) => {
      binaryListenersRef.current.forEach((listener) => listener(payload));
    });
    return () => {
      unsubscribe();
      unsubscribeStatus();
      unsubscribeBinary();
      socket.stop();
      if (socketRef.current === socket) socketRef.current = null;
    };
  }, [code, enabled, token]);

  const sendChat = useCallback(
    (text: string) => socketRef.current?.sendChat(text) ?? false,
    [],
  );
  const setMuted = useCallback(
    (muted: boolean) => socketRef.current?.setMuted(muted) ?? false,
    [],
  );
  const sendBinary = useCallback(
    (payload: ArrayBuffer) => socketRef.current?.sendBinary(payload) ?? false,
    [],
  );
  const subscribeBinary = useCallback(
    (listener: (payload: ArrayBuffer) => void) => {
      binaryListenersRef.current.add(listener);
      return () => binaryListenersRef.current.delete(listener);
    },
    [],
  );
  const subscribeEvent = useCallback(
    (listener: (event: ServerCallEnvelope) => void) => {
      eventListenersRef.current.add(listener);
      return () => eventListenersRef.current.delete(listener);
    },
    [],
  );
  const stop = useCallback(() => socketRef.current?.stop(), []);

  return { status, sendChat, setMuted, sendBinary, subscribeBinary, subscribeEvent, stop };
}
