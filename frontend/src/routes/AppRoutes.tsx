import { useEffect, useRef } from "react";
import { Link, Route, Routes, useLocation } from "react-router-dom";
import { Landing } from "../pages/Landing";
import { Login } from "../pages/Login";
import { Register } from "../pages/Register";
import { Home } from "../pages/Home";
import { CallSetup } from "../pages/CallSetup";
import { Call } from "../pages/Call";

const titles: Record<string, string> = {
  "/": "Sua voz aproxima",
  "/login": "Entrar",
  "/register": "Criar conta",
  "/home": "Suas conversas",
};

export function AppRoutes() {
  const { pathname } = useLocation();
  const previous = useRef(pathname);
  useEffect(() => {
    document.title = `${titles[pathname] ?? (pathname.endsWith("/setup") ? "Pré-chamada" : pathname.startsWith("/call/") ? "Daily meeting" : "Página não encontrada")} | Symphonia`;
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
        <Route path="/home" element={<Home />} />
        <Route path="/call/:id/setup" element={<CallSetup />} />
        <Route path="/call/:id" element={<Call />} />
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
