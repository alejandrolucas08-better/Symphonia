import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";

export function EmailVerification() {
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState("");
  const started = useRef(false);
  const read = useRef(false);
  const navigate = useNavigate();
  const { completeSession } = useAuth();

  useEffect(() => {
    if (read.current) return;
    read.current = true;
    const value = new URLSearchParams(window.location.search).get("token") ?? "";
    window.history.replaceState(window.history.state, "", window.location.pathname);
    setToken(value);
  }, []);

  useEffect(() => {
    if (token === null || started.current) return;
    started.current = true;
    if (!token) { setError("Este link não contém um código válido."); return; }
    api.verifyEmail(token)
      .then(({ data, token: sessionToken }) => {
        if (!sessionToken) throw new Error("missing session token");
        completeSession(data, sessionToken);
        navigate("/home", { replace: true });
      })
      .catch((err) => setError(userFacingError(err)));
  }, [token, completeSession, navigate]);

  return (
    <AuthLayout title="confirme seu" emphasis="E-MAIL.">
      {error ? <p className="error-message" role="alert">{error}</p> : <p role="status">Confirmando seu e-mail...</p>}
      <p className="auth-switch"><Link to="/resend-verification">Solicitar outro link</Link></p>
    </AuthLayout>
  );
}

export function ResendVerification() {
  const [error, setError] = useState("");
  const [sent, setSent] = useState(false);
  const [pending, setPending] = useState(false);
  return (
    <AuthLayout title="confirme seu" emphasis="E-MAIL.">
      <p className="eyebrow form-kicker">REENVIAR CONFIRMAÇÃO</p>
      {sent ? <p role="status">Se houver uma confirmação pendente, enviaremos um novo link para este e-mail.</p> : (
        <form onSubmit={async (event) => {
          event.preventDefault(); setPending(true); setError("");
          const email = String(new FormData(event.currentTarget).get("email") ?? "");
          try { await api.resendVerification(email); setSent(true); }
          catch (err) { setError(userFacingError(err)); }
          finally { setPending(false); }
        }}>
          <div className="field"><label htmlFor="resend-email">e-mail</label><input id="resend-email" name="email" type="email" autoComplete="email" required maxLength={254} /></div>
          {error && <p className="error-message" role="alert">{error}</p>}
          <button className="action-link form-submit" type="submit" disabled={pending}>{pending ? "enviando..." : "Reenviar link"}</button>
        </form>
      )}
      <p className="auth-switch"><Link to="/login">Voltar ao login</Link></p>
    </AuthLayout>
  );
}
