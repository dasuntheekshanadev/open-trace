import type { Anomaly } from '../types';

interface Props {
  anomalies: Anomaly[];
  selected: Anomaly | null;
  onSelect: (a: Anomaly) => void;
}

function formatMetric(metric: string, value: number): string {
  if (metric === 'error_rate') return `${(value * 100).toFixed(1)}%`;
  return `${value.toFixed(0)}ms`;
}

function metricLabel(metric: string): string {
  if (metric === 'error_rate') return 'Error Rate';
  return 'Latency';
}

export function AnomalyFeed({ anomalies, selected, onSelect }: Props) {
  if (anomalies.length === 0) {
    return (
      <div className="anomaly-empty">
        <span className="status-dot status-dot--ok" />
        All edges nominal
      </div>
    );
  }

  return (
    <ul className="anomaly-list">
      {anomalies.map((a, i) => {
        const isSelected =
          selected?.source === a.source &&
          selected?.target === a.target &&
          selected?.metric === a.metric;

        return (
          <li
            key={i}
            className={`anomaly-card${isSelected ? ' anomaly-card--selected' : ''}`}
            onClick={() => onSelect(a)}
          >
            <div className="anomaly-card__header">
              <span className="status-dot status-dot--error" />
              <span className="anomaly-card__edge">
                {a.source} <span className="anomaly-card__arrow">→</span> {a.target}
              </span>
              <span className="anomaly-card__metric">{metricLabel(a.metric)}</span>
            </div>
            <div className="anomaly-card__values">
              <span className="anomaly-card__current">{formatMetric(a.metric, a.value)}</span>
              <span className="anomaly-card__separator">vs</span>
              <span className="anomaly-card__baseline">{formatMetric(a.metric, a.mean)} baseline</span>
            </div>
            <div className="anomaly-card__footer">
              Threshold {formatMetric(a.metric, a.threshold)} · {((a.value - a.mean) / a.stddev).toFixed(1)}σ above normal
            </div>
          </li>
        );
      })}
    </ul>
  );
}
