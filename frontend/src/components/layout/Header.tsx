import { useEffect, useState } from "react";
import { AudioLines, ArrowUpRight, Menu, X } from "lucide-react";
import { Link } from "react-router-dom";

export function Brand({ to = "/" }: { to?: string }) {
  return (
    <Link to={to} className="brand" aria-label="Symphonia, início">
      <AudioLines size={25} strokeWidth={1.8} />
      <span>SYMPHONIA</span>
    </Link>
  );
}

export function Header() {
  const [scrolled, setScrolled] = useState(false);
  const [open, setOpen] = useState(false);
  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);
  return (
    <header className={`public-header ${scrolled ? "scrolled" : ""}`}>
      <div className="container header-inner">
        <Brand />
        <button
          className="icon-button mobile-menu"
          aria-label={open ? "Fechar menu" : "Abrir menu"}
          aria-expanded={open}
          aria-controls="public-nav"
          onClick={() => setOpen(!open)}
        >
          {open ? <X /> : <Menu />}
        </button>
        <nav
          id="public-nav"
          aria-label="Navegação principal"
          className={open ? "nav-open" : ""}
        >
          <a href="#sobre" onClick={() => setOpen(false)}>
            sobre
          </a>
          <a href="#como-funciona" onClick={() => setOpen(false)}>
            como funciona
          </a>
          <Link to="/login">
            entrar <ArrowUpRight size={16} />
          </Link>
        </nav>
      </div>
    </header>
  );
}
