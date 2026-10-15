import { useState } from "react";
import { ArrowUpRight } from "lucide-react";
import { Link, useLocation } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { GoogleSignIn } from "../components/auth/GoogleSignIn";
import { useAuth } from "../hooks/useAuth";
import { userFacingError } from "../services/api";
import { safeReturnTo } from "../utils/navigation";

export function Register() {
  const location = useLocation();
  const { register } = useAuth();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [sentTo, setSentTo] = useState("");
  const returnTo = safeReturnTo(location.state);

  return (
    <AuthLayout title="a conversa" emphasis="COMEÇA AQUI.">
      <p className="eyebrow form-kicker">UM NOVO JEITO DE SE CONECTAR.</p>
      {sentTo ? (
        <div className="auth-result" role="status">
          <h2>Confira seu e-mail</h2>
          <p>Enviamos um link de confirmação para {sentTo}.</p>
          <Link to="/resend-verification">Reenviar confirmação</Link>
        </div>
      ) : (
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          setError("");
          const data = new FormData(event.currentTarget);
          const name = String(data.get("name") ?? "").trim();
          if (!name) {
            setError("Informe seu nome.");
            return;
          }
          if (data.get("password") !== data.get("confirmation")) {
            setError("As senhas precisam ser iguais.");
            return;
          }
          setSubmitting(true);
          try {
            await register({
              name,
              email: String(data.get("email") ?? ""),
              password: String(data.get("password") ?? ""),
            });
            setSentTo(String(data.get("email") ?? ""));
          } catch (err) {
            setError(userFacingError(err));
          } finally {
            setSubmitting(false);
          }
        }}
        onChange={() => setError("")}
      >
        <div className="field">
          <label htmlFor="name">nome</label>
          <input
            id="name"
            name="name"
            autoComplete="name"
            placeholder="Como podemos chamar você?"
            required
            maxLength={60}
          />
        </div>
        <div className="field">
          <label htmlFor="email">email</label>
          <input
            id="email"
            name="email"
            type="email"
            autoComplete="email"
            placeholder="voce@email.com"
            required
            maxLength={254}
          />
        </div>
        <div className="field">
          <label htmlFor="password">senha</label>
          <input
            id="password"
            name="password"
            type="password"
            autoComplete="new-password"
            placeholder="Pelo menos 8 caracteres"
            minLength={8}
            required
          />
        </div>
        <div className="field">
          <label htmlFor="confirmation">confirmar senha</label>
          <input
            id="confirmation"
            name="confirmation"
            type="password"
            autoComplete="new-password"
            placeholder="Mais uma vez"
            minLength={8}
            required
            aria-describedby={error ? "register-error" : undefined}
            aria-invalid={!!error}
          />
        </div>
        {error && (
          <p id="register-error" className="error-message" role="alert">
            {error}
          </p>
        )}
        <button
          className="action-link form-submit"
          type="submit"
          disabled={submitting}
        >
          {submitting ? "criando..." : "Criar conta"}{" "}
          <span className="action-arrow">
            <ArrowUpRight size={22} />
          </span>
        </button>
      </form>
      )}
      {!sentTo && <GoogleSignIn returnTo={returnTo} />}
      <p className="auth-switch">
        já possui conta?
        <br />
        <Link to="/login" state={{ returnTo }}>
          entrar <ArrowUpRight size={15} />
        </Link>
      </p>
    </AuthLayout>
  );
}
