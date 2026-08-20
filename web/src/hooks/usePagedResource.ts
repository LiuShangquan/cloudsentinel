import { useCallback, useEffect, useState } from "react";
import type { Page } from "../types";

export function usePagedResource<T>(loader: (page: number, size: number) => Promise<Page<T>>, pageSize = 20) {
  const [page, setPage] = useState(1);
  const [data, setData] = useState<Page<T> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setData(await loader(page, pageSize));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [loader, page, pageSize]);

  useEffect(() => void load(), [load]);
  return { page, setPage, data, loading, error, reload: load };
}
