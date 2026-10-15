import { useEffect, useState } from "react";
import { api } from "../../services/api";

export function GoogleSignIn({ returnTo }: { returnTo: string }) {
  const [enabled, setEnabled] = useState(false);
  useEffect(() => {
    api.providers().then(({ data }) => setEnabled(data.google)).catch(() => setEnabled(false));
  }, []);
  if (!enabled) return null;
  return <div className="oauth-choice">
    <span>ou</span>
    <button type="button" className="oauth-button" onClick={() => {
      window.location.assign(`/api/auth/google/start?return_to=${encodeURIComponent(returnTo)}`);
    }}>Continuar com Google</button>
  </div>;
}
