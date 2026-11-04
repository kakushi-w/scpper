"use client";

import { useCallback, useEffect, useState } from "react";

export function useLoad<T>(fn: () => Promise<T>, deps: unknown[] = []) {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);

  const run = useCallback(fn, deps);

  const reload = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setData(await run());
    } catch (e) {
      setError(e instanceof Error ? e.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [run]);

  useEffect(() => {
    void reload();
  }, [reload]);

  return { data, error, loading, reload };
}
