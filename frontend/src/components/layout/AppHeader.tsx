import { useEffect, useState } from "react";
import { LogOut } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { useAuth } from "../../hooks/useAuth";
import { useDemo } from "../../contexts/DemoContext";
import { Brand } from "./Header";
import { StatusBadge } from "../ui/StatusBadge";
import { Avatar, initialsFor } from "../ui/Avatar";

export function AppHeader({
  call = false,
  participantCount,
}: {
  call?: boolean;
  participantCount?: number;
}) {
  const { user, logout } = useAuth();
  const { reset } = useDemo();
  const navigate = useNavigate();
  const location = useLocation();
  const [pendingLogout, setPendingLogout] = useState(false);
  const name = user?.name ?? "Usuário";

  useEffect(() => {
    if (pendingLogout && location.pathname === "/") {
      logout();
      reset();
      setPendingLogout(false);
    }
  }, [pendingLogout, location.pathname, logout, reset]);

  return (
    <header className="app-header">
      <div className="container header-inner">
        <Brand to="/home" />
        {call && (
          <span className="room-heading">
            Daily meeting{" "}
            {participantCount !== undefined && (
              <span className="muted">
                / {String(participantCount).padStart(2, "0")}
              </span>
            )}
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
                  navigate("/", { replace: true });
                  setPendingLogout(true);
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
