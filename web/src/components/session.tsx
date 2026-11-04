"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react";
import { api } from "@/lib/api";
import type { SessionUser, Standing } from "@/lib/types";

interface SessionValue {
  user: SessionUser | null;
  standing: Standing | null;
  loading: boolean;
  refresh: () => Promise<void>;
}

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<SessionUser | null>(null);
  const [standing, setStanding] = useState<Standing | null>(null);
  const [loading, setLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const res = await api.me();
      setUser(res.user);
      setStanding(res.standing);
    } catch {
      setUser(null);
      setStanding(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const value = useMemo(() => ({ user, standing, loading, refresh }), [user, standing, loading, refresh]);
  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const ctx = useContext(SessionContext);
  if (!ctx) throw new Error("useSession outside SessionProvider");
  return ctx;
}

export function loginHref(): string {
  if (typeof window === "undefined") return "/-/login";
  return "/-/login?to=" + encodeURIComponent(window.location.pathname + window.location.search);
}
