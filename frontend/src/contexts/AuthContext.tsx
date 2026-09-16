import {
  createContext,
  useCallback,
  useEffect,
  useState,
  type ReactNode,
} from "react";
import { api, ApiError } from "../services/api";
import type { AuthUser, LoginPayload, RegisterPayload } from "../types/auth";

type AuthState = {
  user: AuthUser | null;
  token: string | null;
  loading: boolean;
  login: (payload: LoginPayload) => Promise<void>;
  register: (payload: RegisterPayload) => Promise<void>;
  logout: () => void;
};

export const AuthContext = createContext<AuthState | null>(null);

const SESSION_KEY = "symphonia_token";

function readToken(): string | null {
  try {
    return sessionStorage.getItem(SESSION_KEY);
  } catch {
    return null;
  }
}

function writeToken(token: string | null) {
  try {
    if (token) {
      sessionStorage.setItem(SESSION_KEY, token);
    } else {
      sessionStorage.removeItem(SESSION_KEY);
    }
  } catch {
    // storage unavailable
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [token, setToken] = useState<string | null>(readToken);
  const [loading, setLoading] = useState(true);

  const applyAuth = useCallback((u: AuthUser, t: string) => {
    setUser(u);
    setToken(t);
    writeToken(t);
  }, []);

  const clearAuth = useCallback(() => {
    setUser(null);
    setToken(null);
    writeToken(null);
  }, []);

  useEffect(() => {
    const t = readToken();
    if (!t) {
      setLoading(false);
      return;
    }

    api
      .session(t)
      .then(({ data, token: newToken }) => {
        applyAuth(data, newToken ?? t);
      })
      .catch(() => {
        clearAuth();
      })
      .finally(() => {
        setLoading(false);
      });
  }, [applyAuth, clearAuth]);

  const login = useCallback(
    async (payload: LoginPayload) => {
      const { data, token: t } = await api.login(payload);
      applyAuth(data, t ?? "");
    },
    [applyAuth],
  );

  const register = useCallback(
    async (payload: RegisterPayload) => {
      const { data, token: t } = await api.register(payload);
      applyAuth(data, t ?? "");
    },
    [applyAuth],
  );

  const logout = useCallback(() => {
    clearAuth();
  }, [clearAuth]);

  return (
    <AuthContext.Provider
      value={{ user, token, loading, login, register, logout }}
    >
      {children}
    </AuthContext.Provider>
  );
}


