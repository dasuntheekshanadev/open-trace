interface Props {
  serviceCount: number;
  edgeCount:    number;
  errorRate:    number; // 0–1
  avgLatency:   number; // ms
  anomalyCount: number;
  connected:    boolean;
}

export function StatsBar({ serviceCount, edgeCount, errorRate, avgLatency, anomalyCount, connected }: Props) {
  if (!connected) {
    return (
      <div className="statsbar statsbar--connecting">
        <span className="statsbar__connecting-dot" />
        Connecting to collector…
      </div>
    );
  }

  const errClass = anomalyCount > 0 ? 'stat--error' : errorRate > 0.01 ? 'stat--warn' : 'stat--ok';

  return (
    <div className="statsbar">
      <div className="stat">
        <span className="stat__label">Services</span>
        <span className="stat__value">{serviceCount}</span>
      </div>
      <div className="statsbar__sep" />
      <div className="stat">
        <span className="stat__label">Connections</span>
        <span className="stat__value">{edgeCount}</span>
      </div>
      <div className="statsbar__sep" />
      <div className="stat">
        <span className="stat__label">Avg Latency</span>
        <span className="stat__value">{avgLatency > 0 ? `${avgLatency.toFixed(0)} ms` : '—'}</span>
      </div>
      <div className="statsbar__sep" />
      <div className={`stat ${errClass}`}>
        <span className="stat__label">Error Rate</span>
        <span className="stat__value">{edgeCount > 0 ? `${(errorRate * 100).toFixed(2)}%` : '—'}</span>
      </div>
      <div className="statsbar__sep" />
      <div className={`stat ${anomalyCount > 0 ? 'stat--error' : 'stat--ok'}`}>
        <span className="stat__label">Anomalies</span>
        <span className="stat__value stat__value--mono">{anomalyCount}</span>
      </div>
    </div>
  );
}
