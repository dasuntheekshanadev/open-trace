import { useCallback, useEffect, useState } from 'react';
import { fetchAnomalies, fetchGraph, fetchTimeSeries } from './api/client';
import { AnomalyFeed } from './components/AnomalyFeed';
import { OverviewPanel } from './components/OverviewPanel';
import { RootCausePanel } from './components/RootCausePanel';
import { ServiceGraph } from './components/ServiceGraph';
import { StatsBar } from './components/StatsBar';
import { TimeSeriesChart } from './components/TimeSeriesChart';
import { usePolling } from './hooks/usePolling';
import type { Anomaly, BucketPoint } from './types';

function timeAgo(date: Date | null): string {
  if (!date) return 'connecting...';
  const secs = Math.floor((Date.now() - date.getTime()) / 1000);
  if (secs < 5) return 'just now';
  return `${secs}s ago`;
}

export default function App() {
  const [selectedAnomaly, setSelectedAnomaly] = useState<Anomaly | null>(null);
  const [timeSeries, setTimeSeries] = useState<BucketPoint[]>([]);
  const [tick, setTick] = useState(0);
  const [theme, setTheme] = useState<'dark' | 'light'>(
    () => (localStorage.getItem('opentrace-theme') as 'dark' | 'light') ?? 'dark',
  );
  const toggleTheme = useCallback(() => {
    setTheme(t => {
      const next = t === 'dark' ? 'light' : 'dark';
      localStorage.setItem('opentrace-theme', next);
      return next;
    });
  }, []);

  const { data: graphData, lastUpdated } = usePolling(fetchGraph, 5000);
  const { data: anomalyData } = usePolling(fetchAnomalies, 5000);

  useEffect(() => {
    const id = setInterval(() => setTick(t => t + 1), 1000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    if (!selectedAnomaly) { setTimeSeries([]); return; }
    const { source, target } = selectedAnomaly;
    let cancelled = false;
    const load = () => {
      fetchTimeSeries(source, target)
        .then(d => { if (!cancelled) setTimeSeries(d.buckets); })
        .catch(() => {});
    };
    load();
    const id = setInterval(load, 10000);
    return () => { cancelled = true; clearInterval(id); };
  }, [selectedAnomaly?.source, selectedAnomaly?.target]);

  const edges        = graphData?.edges ?? [];
  const anomalies    = anomalyData?.anomalies ?? [];
  const hasAnomaly   = anomalies.length > 0;
  const connected    = graphData !== null;

  // Compute live stats for the stats bar
  const services     = new Set(edges.flatMap(e => [e.source, e.target]));
  const totalCalls   = edges.reduce((s, e) => s + e.call_count, 0);
  const totalErrors  = edges.reduce((s, e) => s + e.error_count, 0);
  const avgLatency   = edges.length > 0
    ? edges.reduce((s, e) => s + e.avg_latency_ms, 0) / edges.length
    : 0;
  const errorRate    = totalCalls > 0 ? totalErrors / totalCalls : 0;

  const handleEdgeClick = useCallback((source: string, target: string) => {
    const match = anomalies.find(a => a.source === source && a.target === target);
    setSelectedAnomaly(
      match ?? {
        source, target,
        metric: 'error_rate',
        value: 0, mean: 0, stddev: 0, threshold: 0,
        detected_at: new Date().toISOString(),
      },
    );
  }, [anomalies]);

  const selectedEdge = selectedAnomaly
    ? { source: selectedAnomaly.source, target: selectedAnomaly.target }
    : null;

  return (
    <div className="shell" data-theme={theme} data-tick={tick}>
      <header className="topbar">
        <span className="topbar__brand">OpenTrace</span>
        <div className="topbar__status">
          <span className={`status-dot ${hasAnomaly ? 'status-dot--error' : 'status-dot--ok'}`} />
          <span className="topbar__label">
            {hasAnomaly
              ? `${anomalies.length} anomaly${anomalies.length > 1 ? 's' : ''} detected`
              : 'All systems nominal'}
          </span>
          <span className="topbar__updated">Updated {timeAgo(lastUpdated)}</span>
        </div>
        <button className="theme-toggle" onClick={toggleTheme} title={theme === 'dark' ? 'Light mode' : 'Dark mode'}>
          {theme === 'dark' ? (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="5"/><line x1="12" y1="1" x2="12" y2="3"/><line x1="12" y1="21" x2="12" y2="23"/><line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/><line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/><line x1="1" y1="12" x2="3" y2="12"/><line x1="21" y1="12" x2="23" y2="12"/><line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/><line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
          ) : (
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          )}
        </button>
      </header>

      <StatsBar
        serviceCount={services.size}
        edgeCount={edges.length}
        errorRate={errorRate}
        avgLatency={avgLatency}
        anomalyCount={anomalies.length}
        connected={connected}
      />

      <div className="workspace">
        <section className="pane pane--graph">
          <div className="pane__title">
            Service Graph
            {edges.length > 0 && (
              <span className="pane__meta">
                {services.size} services · {edges.length} connections
              </span>
            )}
          </div>
          <ServiceGraph
            data={graphData}
            anomalies={anomalies}
            onEdgeClick={handleEdgeClick}
            selectedEdge={selectedEdge}
            theme={theme}
          />
        </section>

        <aside className="sidebar">
          <section className="pane pane--anomalies">
            <div className="pane__title">
              Anomalies
              {hasAnomaly && <span className="pane__badge">{anomalies.length}</span>}
            </div>
            <AnomalyFeed
              anomalies={anomalies}
              selected={selectedAnomaly}
              onSelect={setSelectedAnomaly}
            />
          </section>

          {selectedAnomaly ? (
            <section className="pane pane--rootcause">
              <div className="pane__title">
                {selectedAnomaly.source} → {selectedAnomaly.target}
                <button className="pane__close" onClick={() => setSelectedAnomaly(null)}>
                  Dismiss
                </button>
              </div>
              <div style={{ padding: '12px 16px 0' }}>
                <TimeSeriesChart buckets={timeSeries} />
              </div>
              <RootCausePanel anomaly={selectedAnomaly} />
            </section>
          ) : (
            <section className="pane pane--overview">
              <div className="pane__title">Overview</div>
              <OverviewPanel edges={edges} />
            </section>
          )}
        </aside>
      </div>
    </div>
  );
}
