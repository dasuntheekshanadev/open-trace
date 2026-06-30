import { useEffect, useState } from 'react';

export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number,
): { data: T | null; error: string | null; lastUpdated: Date | null } {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);

  useEffect(() => {
    let cancelled = false;

    const run = () =>
      fetcher()
        .then(d => {
          if (!cancelled) {
            setData(d);
            setError(null);
            setLastUpdated(new Date());
          }
        })
        .catch(e => {
          if (!cancelled) setError((e as Error).message);
        });

    run();
    const id = setInterval(run, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, [intervalMs]); // eslint-disable-line react-hooks/exhaustive-deps

  return { data, error, lastUpdated };
}
