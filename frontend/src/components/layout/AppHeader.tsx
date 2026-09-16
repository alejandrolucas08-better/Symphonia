import { LogOut } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useDemo } from "../../contexts/DemoContext";
import { Brand } from "./Header";
import { StatusBadge } from "../ui/StatusBadge";
import { Avatar, initialsFor } from "../ui/Avatar";

export function AppHeader({ call = false }: { call?: boolean }) {
  const { name, reset } = useDemo();
  const navigate = useNavigate();
  return (
    <header className="app-header">
      <div className="container header-inner">
        <Brand to="/home" />
        {call && (
          <span className="room-heading">
            Daily meeting <span className="muted">/ 02</span>
          </span>
        )}
        <div className="header-account">
          {call ? (
            <StatusBadge>
              conectado <span className="demo-tag">demo</span>
            </StatusBadge>
          ) : (
            <>
              <span className="account-name">{name}</span>
              <Avatar initials={initialsFor(name)} small />
              <button
                className="icon-button"
                aria-label="Sair"
                title="Sair"
                onClick={() => {
                  reset();
                  navigate("/");
                }}
              >
                <LogOut size={18} />
              </button>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
