import { Link } from "react-router-dom";
import { ArrowLeft } from "lucide-react";
import type { ReactNode } from "react";
import { Brand } from "./Header";
import { AudioWave } from "../ui/AudioWave";

export function AuthLayout({
  title,
  emphasis,
  children,
}: {
  title: string;
  emphasis: string;
  children: ReactNode;
}) {
  return (
    <div className="auth-page">
      <header className="container header-inner">
        <Brand />
        <Link className="subtle-link" to="/">
          <ArrowLeft size={16} /> voltar
        </Link>
      </header>
      <main id="main-content" className="container auth-layout">
        <div className="auth-intro">
          <p className="eyebrow">
            <span className="tiny-line" /> A CONVERSA CONTINUA
          </p>
          <h1>
            {title}
            <br />
            <strong>{emphasis}</strong>
          </h1>
          <div className="auth-wave">
            <AudioWave large />
          </div>
          <p className="muted">sua voz. novas possibilidades.</p>
        </div>
        <section className="auth-form-area">{children}</section>
      </main>
      <footer className="container simple-footer">
        <span>SYMPHONIA © {new Date().getFullYear()}</span>
        <span>DUAS VOZES. UMA CONVERSA.</span>
      </footer>
    </div>
  );
}
