import { useState } from "react";
import {
  ArrowDown,
  ArrowRight,
  ArrowUpRight,
  AudioLines,
  Pause,
  Play,
  SkipForward,
} from "lucide-react";
import { Link } from "react-router-dom";
import { Header, Brand } from "../components/layout/Header";
import { ActionLink } from "../components/ui/Button";
import { AudioWave, SignalScene } from "../components/ui/AudioWave";
import { Reveal } from "../components/ui/Reveal";
import { StatusBadge } from "../components/ui/StatusBadge";
import { useConversation } from "../hooks/useConversation";
import { transcripts } from "../data/mocks";

function ConversationDemo() {
  const { turn, key, ready, animated, paused, setPaused, next } =
    useConversation();
  const phrase = transcripts[turn];
  return (
    <div className="conversation-demo">
      <div className="demo-top">
        <span className="eyebrow">UMA CONVERSA, DOIS IDIOMAS</span>
        <div className="demo-controls">
          <span className="demo-tag">simulação</span>
          <button
            className="icon-button"
            title={paused ? "Reproduzir demonstração" : "Pausar demonstração"}
            aria-label={
              paused ? "Reproduzir demonstração" : "Pausar demonstração"
            }
            onClick={() => setPaused(!paused)}
          >
            {paused ? <Play size={17} /> : <Pause size={17} />}
          </button>
          <button
            className="icon-button"
            title="Próxima fala"
            aria-label="Próxima fala"
            onClick={next}
          >
            <SkipForward size={17} />
          </button>
        </div>
      </div>
      <div className="demo-people">
        <div>
          <p className="eyebrow">PT-BR</p>
          <h3>Alejandro</h3>
          <StatusBadge active={turn === 0}>
            {turn === 0 ? "falando" : "ouvindo"}
          </StatusBadge>
        </div>
        <div className="demo-signal">
          <AudioWave active={animated} />
          <ArrowRight size={20} className={turn === 1 ? "reverse-arrow" : ""} />
        </div>
        <div>
          <p className="eyebrow">EN-US</p>
          <h3>Mateus</h3>
          <StatusBadge active={turn === 1}>
            {turn === 1 ? "falando" : "ouvindo"}
          </StatusBadge>
        </div>
      </div>
      <div className="demo-quote" key={key}>
        <p className="muted">“{phrase.original}”</p>
        <p className={`translated ${ready ? "translation-ready" : ""}`}>
          “{phrase.translated}”
        </p>
      </div>
    </div>
  );
}

export function Landing() {
  const [paused, setPaused] = useState(false);
  return (
    <div className={`landing ${paused ? "motion-paused" : ""}`}>
      <Header />
      <main id="main-content">
        <section className="hero">
          <div className="container hero-content">
            <div className="hero-kicker">
              <span className="eyebrow">
                <span className="status-dot" /> VOZ SEM FRONTEIRAS
              </span>
              <span className="eyebrow muted">PT ⇄ EN / E ALÉM</span>
            </div>
            <h1>
              SYMPHONIA<span className="brand-period">.</span>
            </h1>
            <div className="hero-message">
              <div>
                <p className="hero-lead">fale no seu idioma.</p>
                <h2>
                  escute no
                  <br />
                  <strong>SEU IDIOMA.</strong>
                </h2>
              </div>
              <div className="hero-aside">
                <p>
                  duas pessoas.
                  <br />
                  dois idiomas.
                  <br />
                  <span>uma conversa.</span>
                </p>
                <ActionLink to="/register">Começar conversa</ActionLink>
              </div>
            </div>
          </div>
          <div className="hero-signal">
            <SignalScene paused={paused} />
          </div>
          <div className="container hero-bottom">
            <a className="subtle-link" href="#sobre">
              <ArrowDown size={16} /> explore a conexão
            </a>
            <button
              className="subtle-button"
              onClick={() => setPaused(!paused)}
              aria-label={paused ? "Retomar movimento" : "Pausar movimento"}
            >
              {paused ? <Play size={14} /> : <Pause size={14} />}
              <span>{paused ? "retomar movimento" : "pausar movimento"}</span>
            </button>
          </div>
        </section>

        <section id="sobre" className="editorial-section container">
          <Reveal className="section-grid">
            <div className="section-index">
              <span>01</span>
              <h2>SOBRE</h2>
            </div>
            <div className="about-copy">
              <p>
                comunicação deveria
                <br />
                <strong>aproximar pessoas.</strong>
              </p>
              <p className="about-secondary">
                não importa qual idioma
                <br />
                cada pessoa fala.
              </p>
              <div className="about-foot">
                <span className="tiny-line" />
                <p>
                  O mundo tem muitas vozes.
                  <br />A conexão começa quando a gente se entende.
                </p>
              </div>
            </div>
          </Reveal>
        </section>

        <section id="como-funciona" className="process-section">
          <div className="container">
            <Reveal>
              <div className="section-grid">
                <div className="section-index">
                  <span>02</span>
                  <h2>COMO FUNCIONA</h2>
                </div>
                <div>
                  <h3 className="section-title">
                    a sua voz.
                    <br />
                    <em>um novo alcance.</em>
                  </h3>
                  <p className="muted section-description">
                    Você fala. A conversa encontra o caminho.
                  </p>
                </div>
              </div>
              <div className="process-flow">
                {[
                  {
                    title: "FALA",
                    detail: "Tudo começa com você.",
                    code: "01 / ORIGEM",
                  },
                  {
                    title: "TRANSCRIÇÃO",
                    detail: "Sua voz ganha palavras.",
                    code: "02 / PALAVRA",
                  },
                  {
                    title: "TRADUÇÃO",
                    detail: "O sentido atravessa idiomas.",
                    code: "03 / CONEXÃO",
                  },
                  {
                    title: "VOZ",
                    detail: "A outra pessoa entende.",
                    code: "04 / ENCONTRO",
                  },
                ].map((step, index) => (
                  <div
                    className="process-step"
                    key={step.title}
                    style={{ transitionDelay: `${index * 120}ms` }}
                  >
                    <span className="eyebrow">{step.code}</span>
                    <div className="process-line">
                      <span className="process-point" />
                      {index < 3 && <ArrowRight size={18} />}
                    </div>
                    <h4>{step.title}</h4>
                    <p>{step.detail}</p>
                  </div>
                ))}
              </div>
            </Reveal>
          </div>
        </section>

        <section id="demonstracao" className="editorial-section container">
          <Reveal>
            <div className="section-grid demo-heading">
              <div className="section-index">
                <span>03</span>
                <h2>EM SINTONIA</h2>
              </div>
              <h3 className="section-title">
                idiomas diferentes.
                <br />
                <em>mesma frequência.</em>
              </h3>
            </div>
            <ConversationDemo />
          </Reveal>
        </section>

        <section className="final-cta">
          <div className="container">
            <Reveal>
              <AudioLines size={32} strokeWidth={1.3} />
              <h2>
                pronto para conversar
                <br />
                <strong>sem barreiras?</strong>
              </h2>
              <ActionLink to="/register">Iniciar chamada</ActionLink>
            </Reveal>
          </div>
        </section>
      </main>
      <footer className="container landing-footer">
        <Brand />
        <span>DUAS VOZES. UMA CONVERSA.</span>
        <Link to="/login">
          vamos conversar <ArrowUpRight size={16} />
        </Link>
        <span className="copyright">
          © {new Date().getFullYear()} Symphonia
        </span>
      </footer>
    </div>
  );
}
