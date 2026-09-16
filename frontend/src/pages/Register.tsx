import { useState } from "react";
import { ArrowUpRight } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { useDemo } from "../contexts/DemoContext";

export function Register() {
  const navigate = useNavigate();
  const { setName } = useDemo();
  const [error, setError] = useState("");
  return (
    <AuthLayout title="a conversa" emphasis="COMEÇA AQUI.">
      <p className="eyebrow form-kicker">UM NOVO JEITO DE SE CONECTAR.</p>
      <form
        onSubmit={(event) => {
          event.preventDefault();
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
          setName(name);
          navigate("/home");
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
        <button className="action-link form-submit" type="submit">
          Criar conta{" "}
          <span className="action-arrow">
            <ArrowUpRight size={22} />
          </span>
        </button>
        <p className="form-note">
          Ambiente demonstrativo. Use dados fictícios.
        </p>
      </form>
      <p className="auth-switch">
        já possui conta?
        <br />
        <Link to="/login">
          entrar <ArrowUpRight size={15} />
        </Link>
      </p>
    </AuthLayout>
  );
}
