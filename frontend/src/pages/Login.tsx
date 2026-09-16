import { useState } from "react";
import { ArrowUpRight } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { useAuth } from "../hooks/useAuth";
import { userFacingError } from "../services/api";

export function Login() {
  const navigate = useNavigate();
  const { login } = useAuth();
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  return (
    <AuthLayout title="entre na sua" emphasis="CONTA.">
      <p className="eyebrow form-kicker">BOM TER VOCÊ POR AQUI.</p>
      <form
        onSubmit={async (event) => {
          event.preventDefault();
          setError("");
          setSubmitting(true);
          try {
            const data = new FormData(event.currentTarget);
            await login({
              email: String(data.get("email") ?? ""),
              password: String(data.get("password") ?? ""),
            });
            navigate("/home");
          } catch (err) {
            setError(userFacingError(err));
          } finally {
            setSubmitting(false);
          }
        }}
        onChange={() => setError("")}
      >
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
            autoComplete="current-password"
            placeholder="Sua senha"
            required
          />
        </div>
        {error && (
          <p className="error-message" role="alert">
            {error}
          </p>
        )}
        <button
          className="action-link form-submit"
          type="submit"
          disabled={submitting}
        >
          {submitting ? "entrando..." : "Entrar"}{" "}
          <span className="action-arrow">
            <ArrowUpRight size={22} />
          </span>
        </button>
      </form>
      <p className="auth-switch">
        ainda não possui conta?
        <br />
        <Link to="/register">
          criar conta <ArrowUpRight size={15} />
        </Link>
      </p>
    </AuthLayout>
  );
}
