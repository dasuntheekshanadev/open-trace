import type { AnomalyData, GraphData, RootCauseData, TimeSeriesData } from '../types';

const BASE = (import.meta.env.VITE_COLLECTOR_URL as string | undefined) ?? '';

export async function fetchGraph(): Promise<GraphData> {
  const res = await fetch(`${BASE}/graph`);
  if (!res.ok) throw new Error('Failed to fetch graph');
  return res.json();
}

export async function fetchAnomalies(): Promise<AnomalyData> {
  const res = await fetch(`${BASE}/anomalies`);
  if (!res.ok) throw new Error('Failed to fetch anomalies');
  return res.json();
}

export async function fetchRootCause(source: string, target: string): Promise<RootCauseData> {
  const res = await fetch(`${BASE}/rootcause?source=${encodeURIComponent(source)}&target=${encodeURIComponent(target)}`);
  if (!res.ok) throw new Error('Failed to fetch root cause');
  return res.json();
}

export async function fetchTimeSeries(source: string, target: string): Promise<TimeSeriesData> {
  const res = await fetch(`${BASE}/timeseries?source=${encodeURIComponent(source)}&target=${encodeURIComponent(target)}`);
  if (!res.ok) throw new Error('Failed to fetch time series');
  return res.json();
}
