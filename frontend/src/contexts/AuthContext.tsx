"use client";

import { createContext, useContext, useEffect, useState, useCallback, type ReactNode } from "react";
import { authAPI } from "@/lib/api";
import type { User, AuthProviderInfo } from "@/lib/types";

interface AuthState {
  user: User | null;
  isAuthenticated: boolean;
  setupRequired: boolean;
  loading: boolean;
  providers: AuthProviderInfo[];
  login: (username: string, password: string, provider?: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [setupRequired, setSetupRequired] = useState(false);
  const [loading, setLoading] = useState(true);
  const [providers, setProviders] = useState<AuthProviderInfo[]>([]);

  const refresh = useCallback(async () => {
    try {
      const status = await authAPI.status();
      setSetupRequired(status.setup_required);
      setIsAuthenticated(status.authenticated);
      setProviders(status.providers ?? []);

      if (status.authenticated) {
        const me = await authAPI.me();
        setUser(me);
      } else {
        setUser(null);
      }
    } catch {
      setUser(null);
      setIsAuthenticated(false);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const login = async (username: string, password: string, provider?: string) => {
    await authAPI.login({ username, password, provider: provider || "local" });
    await refresh();
  };

  const logout = async () => {
    await authAPI.logout();
    setUser(null);
    setIsAuthenticated(false);
  };

  return (
    <AuthContext.Provider value={{ user, isAuthenticated, setupRequired, loading, providers, login, logout, refresh }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
