import { useEffect, useRef, type CSSProperties } from "react";
import { useReducedMotion } from "../../hooks/useReducedMotion";

export function AudioWave({
  active = true,
  large = false,
}: {
  active?: boolean;
  large?: boolean;
}) {
  return (
    <div
      className={`audio-wave ${active ? "wave-active" : ""} ${large ? "wave-large" : ""}`}
      aria-hidden="true"
    >
      {Array.from({ length: large ? 57 : 29 }, (_, index) => (
        <span
          key={index}
          style={
            {
              "--height": `${14 + Math.abs(Math.sin(index * 0.73) * Math.cos(index * 0.27)) * 86}%`,
              "--delay": `${index * -0.13}s`,
              "--duration": `${1.2 + (index % 5) * 0.2}s`,
            } as CSSProperties
          }
        />
      ))}
    </div>
  );
}

// A procedural signal stays sharp at any width and never requests audio access.
export function SignalScene({ paused = false }: { paused?: boolean }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const reduced = useReducedMotion();
  useEffect(() => {
    const canvas = canvasRef.current;
    const context = canvas?.getContext("2d");
    if (!canvas || !context) return;
    let frame = 0;
    let width = 0;
    let height = 0;
    let visible = true;
    let pointer = 0;
    const draw = (time: number) => {
      const phase = reduced || paused ? 0 : time * 0.00022;
      context.clearRect(0, 0, width, height);
      for (let layer = 0; layer < 22; layer++) {
        context.beginPath();
        context.lineWidth = layer % 5 === 0 ? 1.3 : 0.7;
        context.strokeStyle =
          layer < 13
            ? `rgba(49,92,114,${0.2 + layer * 0.024})`
            : "rgba(180,91,64,0.35)";
        for (let x = 0; x <= width; x += 3) {
          const n = x / width;
          const envelope = Math.sin(n * Math.PI) ** 1.7;
          const frequency = Math.sin(n * 18 + phase + layer * 0.075);
          const y =
            height / 2 +
            frequency *
              Math.cos(n * 5 - phase * 0.8 + layer * 0.13) *
              envelope *
              height *
              (0.33 + pointer * 0.035) +
            (layer - 11) * 2.8;
          if (x === 0) context.moveTo(x, y);
          else context.lineTo(x, y);
        }
        context.stroke();
      }
      if (!reduced && !paused && visible && !document.hidden)
        frame = requestAnimationFrame(draw);
    };
    const restart = () => {
      cancelAnimationFrame(frame);
      draw(performance.now());
    };
    const resize = new ResizeObserver(() => {
      width = canvas.clientWidth;
      height = canvas.clientHeight;
      const ratio = Math.min(window.devicePixelRatio, 2);
      canvas.width = width * ratio;
      canvas.height = height * ratio;
      context.setTransform(ratio, 0, 0, ratio, 0, 0);
      restart();
    });
    const visibility = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      restart();
    });
    const move = (event: PointerEvent) => {
      pointer = event.offsetX / Math.max(width, 1);
    };
    resize.observe(canvas);
    visibility.observe(canvas);
    canvas.addEventListener("pointermove", move);
    document.addEventListener("visibilitychange", restart);
    return () => {
      cancelAnimationFrame(frame);
      resize.disconnect();
      visibility.disconnect();
      canvas.removeEventListener("pointermove", move);
      document.removeEventListener("visibilitychange", restart);
    };
  }, [reduced, paused]);
  return <canvas ref={canvasRef} className="signal-scene" aria-hidden="true" />;
}
