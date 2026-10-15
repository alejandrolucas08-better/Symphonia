import { useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { api, userFacingError } from "../services/api";

export function ForgotPassword() {
  const [sent, setSent] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  return (
    <AuthLayout title="recupere sua" emphasis="SENHA.">
      <p className="eyebrow form-kicker">ACESSO À CONTA</p>
      {sent ? <p role="status">Se este e-mail tiver uma conta, enviaremos um link para redefinir a senha.</p> : (
        <form onSubmit={async (event) => {
          event.preventDefault(); setPending(true); setError("");
          const email = String(new FormData(event.currentTarget).get("email") ?? "");
          try { await api.forgotPassword(email); setSent(true); }
          catch (err) { setError(userFacingError(err)); }
          finally { setPending(false); }
        }}>
          <div className="field"><label htmlFor="recovery-email">e-mail</label><input id="recovery-email" name="email" type="email" autoComplete="email" required maxLength={254} /></div>
          {error && <p className="error-message" role="alert">{error}</p>}
          <button className="action-link form-submit" type="submit" disabled={pending}>{pending ? "enviando..." : "Enviar link"}</button>
        </form>
      )}
      <p className="auth-switch"><Link to="/login">Voltar ao login</Link></p>
    </AuthLayout>
  );
}

export function ResetPassword() {
  const [token, setToken] = useState<string | null>(null);
  const read = useRef(false);
  useEffect(() => {
    if (read.current) return;
    read.current = true;
    const value = new URLSearchParams(window.location.search).get("token") ?? "";
    window.history.replaceState(window.history.state, "", window.location.pathname);
    setToken(value);
  }, []);
  const [done, setDone] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  return (
    <AuthLayout title="nova" emphasis="SENHA.">
      {token === null ? <p role="status">Carregando...</p> : !token ? <p className="error-message" role="alert">Este link não contém um código válido.</p> : done ? (
        <div role="status"><p>Senha atualizada. Entre novamente com sua nova senha.</p><Link to="/login">Ir para o login</Link></div>
      ) : (
        <form onSubmit={async (event) => {
          event.preventDefault(); setError("");
          const data = new FormData(event.currentTarget);
          if (data.get("password") !== data.get("confirmation")) { setError("As senhas precisam ser iguais."); return; }
          setPending(true);
          try { await api.resetPassword(token, String(data.get("password") ?? "")); setDone(true); }
          catch (err) { setError(userFacingError(err)); }
          finally { setPending(false); }
        }}>
          <div className="field"><label htmlFor="new-password">nova senha</label><input id="new-password" name="password" type="password" autoComplete="new-password" minLength={8} required /></div>
          <div className="field"><label htmlFor="confirm-new-password">confirmar nova senha</label><input id="confirm-new-password" name="confirmation" type="password" autoComplete="new-password" minLength={8} required /></div>
          {error && <p className="error-message" role="alert">{error}</p>}
          <button className="action-link form-submit" type="submit" disabled={pending}>{pending ? "salvando..." : "Salvar senha"}</button>
        </form>
      )}
      <p className="auth-switch"><Link to="/login">Voltar ao login</Link></p>
    </AuthLayout>
  );
}
