import { useEffect, useState } from "react";
import { useReducedMotion } from "./useReducedMotion";

export function useConversation() {
  const reduced = useReducedMotion();
  const [index, setIndex] = useState(0);
  const [paused, setPaused] = useState(false);
  const [ready, setReady] = useState(reduced);
  useEffect(() => {
    if (reduced || paused) return;
    const timer = window.setInterval(
      () => setIndex((value) => value + 1),
      7000,
    );
    return () => window.clearInterval(timer);
  }, [paused, reduced]);
  useEffect(() => {
    setReady(false);
    if (reduced || paused) return;
    const timer = window.setTimeout(() => setReady(true), 1200);
    return () => window.clearTimeout(timer);
  }, [index, reduced, paused]);
  return {
    turn: index % 2,
    key: index,
    ready: reduced || paused || ready,
    animated: !reduced && !paused,
    paused,
    setPaused,
    next: () => setIndex((value) => value + 1),
  };
}
