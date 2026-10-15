import { useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import { AppHeader } from "../components/layout/AppHeader";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";

export function ChangePassword() {
  const { user, token, replaceToken } = useAuth();
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [pending, setPending] = useState(false);
  const [googleAvailable, setGoogleAvailable] = useState(false);
  const [linkPending, setLinkPending] = useState(false);
  const [linkError, setLinkError] = useState("");
  const location = useLocation();
  const googleError = new URLSearchParams(location.search).get("google_error");

  useEffect(() => {
    api.providers().then(({ data }) => setGoogleAvailable(data.google)).catch(() => setGoogleAvailable(false));
  }, []);
  return <>
    <AppHeader />
    <main id="main-content" className="container account-page">
      <p className="eyebrow">SUA CONTA</p>
      <h1>Alterar senha</h1>
      <p className="muted">{user?.email}</p>
      {googleError && <p className="error-message" role="alert">{googleError === "google_email_mismatch" ? "Use no Google o mesmo e-mail desta conta." : "Não foi possível vincular esta conta ao Google."}</p>}
      {user?.google_linked ? <p className="account-provider-status">Google vinculado</p> : googleAvailable && <button className="oauth-button account-link-google" type="button" disabled={linkPending} onClick={async () => {
        if (!token) return;
        setLinkPending(true); setLinkError("");
        try {
          const { data } = await api.startGoogleLink(token);
          window.location.assign(data.url);
        } catch (err) {
          setLinkError(userFacingError(err)); setLinkPending(false);
        }
      }}>{linkPending ? "Conectando..." : "Vincular Google"}</button>}
      {linkError && <p className="error-message" role="alert">{linkError}</p>}
      {!user?.has_password ? <p>Esta conta usa o Google. <Link to="/forgot-password">Defina uma senha por e-mail</Link>.</p> : (
        <form onSubmit={async (event) => {
          event.preventDefault(); setError(""); setSuccess(false);
          const form = event.currentTarget;
          const data = new FormData(form);
          if (data.get("new_password") !== data.get("confirmation")) { setError("As senhas precisam ser iguais."); return; }
          if (!token) return;
          setPending(true);
          try {
            const result = await api.changePassword(token, String(data.get("current_password") ?? ""), String(data.get("new_password") ?? ""));
            if (!result.token) throw new Error("missing session token");
            replaceToken(result.token);
            form.reset(); setSuccess(true);
          } catch (err) { setError(userFacingError(err)); }
          finally { setPending(false); }
        }}>
          <div className="field"><label htmlFor="current-password">senha atual</label><input id="current-password" name="current_password" type="password" autoComplete="current-password" required /></div>
          <div className="field"><label htmlFor="account-new-password">nova senha</label><input id="account-new-password" name="new_password" type="password" autoComplete="new-password" minLength={8} required /></div>
          <div className="field"><label htmlFor="account-confirm-password">confirmar nova senha</label><input id="account-confirm-password" name="confirmation" type="password" autoComplete="new-password" minLength={8} required /></div>
          {error && <p className="error-message" role="alert">{error}</p>}
          {success && <p role="status">Senha alterada com sucesso.</p>}
          <button className="action-link form-submit" type="submit" disabled={pending}>{pending ? "salvando..." : "Salvar senha"}</button>
        </form>
      )}
    </main>
  </>;
}
