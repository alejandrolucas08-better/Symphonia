import { ArrowDown } from "lucide-react";

export function Transcript({
  speaker,
  original,
  translated,
  source,
  target,
  ready,
  turnKey,
}: {
  speaker: string;
  original: string;
  translated: string;
  source: string;
  target: string;
  ready: boolean;
  turnKey: number;
}) {
  return (
    <section className="transcript" aria-label="Transcrição">
      <div className="panel-heading">
        <h2>TRANSCRIÇÃO</h2>
      </div>
      <div className="transcript-content" key={turnKey}>
        <p className="transcript-attribution">
          {speaker} <span>· {source}</span>
        </p>
        <p className="original-text">{original}</p>
        <span className="translation-label">
          <ArrowDown size={14} /> {target}
        </span>
        <p className={`translated-text ${ready ? "translation-ready" : ""}`}>
          {translated}
        </p>
      </div>
    </section>
  );
}
