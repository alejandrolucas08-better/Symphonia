import { ArrowUpRight } from "lucide-react";
import { Link, useNavigate } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { useDemo } from "../contexts/DemoContext";

export function Login() {
  const navigate = useNavigate();
  const { setName } = useDemo();
  return (
    <AuthLayout title="entre na sua" emphasis="CONTA.">
      <p className="eyebrow form-kicker">BOM TER VOCÊ POR AQUI.</p>
      <form
        onSubmit={(event) => {
          event.preventDefault();
          setName("Alejandro");
          navigate("/home");
        }}
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
        <button className="action-link form-submit" type="submit">
          Entrar{" "}
          <span className="action-arrow">
            <ArrowUpRight size={22} />
          </span>
        </button>
        <p className="form-note">
          Ambiente demonstrativo. Use dados fictícios.
        </p>
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
