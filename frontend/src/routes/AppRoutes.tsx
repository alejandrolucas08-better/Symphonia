import { useEffect, useRef } from "react";
import { Link, Route, Routes, useLocation } from "react-router-dom";
import { Landing } from "../pages/Landing";
import { Login } from "../pages/Login";
import { Register } from "../pages/Register";
import { EmailVerification, ResendVerification } from "../pages/EmailVerification";
import { ForgotPassword, ResetPassword } from "../pages/PasswordRecovery";
import { GoogleComplete } from "../pages/GoogleComplete";
import { ChangePassword } from "../pages/ChangePassword";
import { Home } from "../pages/Home";
import { CallSetup } from "../pages/CallSetup";
import { Call } from "../pages/Call";
import { ProtectedRoute } from "../components/auth/ProtectedRoute";

const titles: Record<string, string> = {
  "/": "Sua voz aproxima",
  "/login": "Entrar",
  "/register": "Criar conta",
  "/verify-email": "Confirmar e-mail",
  "/resend-verification": "Reenviar confirmação",
  "/forgot-password": "Recuperar senha",
  "/reset-password": "Redefinir senha",
  "/auth/google/complete": "Entrar com Google",
  "/account/password": "Alterar senha",
  "/home": "Suas conversas",
};

export function AppRoutes() {
  const { pathname } = useLocation();
  const previous = useRef(pathname);
  useEffect(() => {
    document.title = `${titles[pathname] ?? (pathname.endsWith("/setup") ? "Pré-chamada" : pathname.startsWith("/call/") ? "Reunião" : "Página não encontrada")} | Symphonia`;
    window.scrollTo(0, 0);
    if (previous.current !== pathname) {
      const main = document.getElementById("main-content");
      main?.setAttribute("tabindex", "-1");
      main?.focus({ preventScroll: true });
      previous.current = pathname;
    }
  }, [pathname]);
  return (
    <>
      <a href="#main-content" className="skip-link">
        Pular para o conteúdo
      </a>
      <Routes>
        <Route path="/" element={<Landing />} />
        <Route path="/login" element={<Login />} />
        <Route path="/register" element={<Register />} />
        <Route path="/verify-email" element={<EmailVerification />} />
        <Route path="/resend-verification" element={<ResendVerification />} />
        <Route path="/forgot-password" element={<ForgotPassword />} />
        <Route path="/reset-password" element={<ResetPassword />} />
        <Route path="/auth/google/complete" element={<GoogleComplete />} />
        <Route path="/account/password" element={<ProtectedRoute><ChangePassword /></ProtectedRoute>} />
        <Route
          path="/home"
          element={
            <ProtectedRoute>
              <Home />
            </ProtectedRoute>
          }
        />
        <Route
          path="/call/:id/setup"
          element={
            <ProtectedRoute>
              <CallSetup />
            </ProtectedRoute>
          }
        />
        <Route
          path="/call/:id"
          element={
            <ProtectedRoute>
              <Call />
            </ProtectedRoute>
          }
        />
        <Route
          path="*"
          element={
            <main id="main-content" className="container not-found">
              <p className="eyebrow">404 / FORA DE SINTONIA</p>
              <h1>Página não encontrada.</h1>
              <Link className="action-link" to="/">
                Voltar ao início
              </Link>
            </main>
          }
        />
      </Routes>
    </>
  );
}
