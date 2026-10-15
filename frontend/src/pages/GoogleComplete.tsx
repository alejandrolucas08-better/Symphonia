import { useEffect, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { AuthLayout } from "../components/layout/AuthLayout";
import { useAuth } from "../hooks/useAuth";
import { api, userFacingError } from "../services/api";
import { safeReturnTo } from "../utils/navigation";

export function GoogleComplete() {
  const [params, setParams] = useState<{ ticket: string; returnTo: string } | null>(null);
  const read = useRef(false);
  useEffect(() => {
    if (read.current) return;
    read.current = true;
    const params = new URLSearchParams(window.location.search);
    const value = { ticket: params.get("ticket") ?? "", returnTo: params.get("return_to") ?? "/home" };
    window.history.replaceState(window.history.state, "", window.location.pathname);
    setParams(value);
  }, []);
  const started = useRef(false);
  const [error, setError] = useState("");
  const { completeSession } = useAuth();
  const navigate = useNavigate();

  useEffect(() => {
    if (!params || started.current) return;
    started.current = true;
    if (!params.ticket) { setError("O acesso pelo Google não foi concluído."); return; }
    api.finishGoogle(params.ticket).then(({ data, token }) => {
      if (!token) throw new Error("missing session token");
      completeSession(data, token);
      navigate(safeReturnTo({ returnTo: params.returnTo }), { replace: true });
    }).catch((err) => setError(userFacingError(err)));
  }, [params, completeSession, navigate]);

  return <AuthLayout title="entrando com" emphasis="GOOGLE.">
    {error ? <p className="error-message" role="alert">{error}</p> : <p role="status">Concluindo o acesso...</p>}
    <p className="auth-switch"><Link to="/login">Voltar ao login</Link></p>
  </AuthLayout>;
}
