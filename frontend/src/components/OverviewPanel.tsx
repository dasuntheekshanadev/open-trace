import type { Edge } from '../types';

interface Props {
  edges: Edge[];
}

export function OverviewPanel({ edges }: Props) {
  if (edges.length === 0) {
    return (
      <div className="overview-empty">
        <div className="overview-empty__ring" />
        <div className="overview-empty__text">Waiting for traces</div>
        <div className="overview-empty__sub">Start sending traffic to see the service graph populate</div>
      </div>
    );
  }

  const sorted = [...edges].sort((a, b) => b.avg_latency_ms - a.avg_latency_ms);
  const errorEdges = edges.filter(e => e.error_rate > 0).sort((a, b) => b.error_rate - a.error_rate);
  const maxLatency = sorted[0]?.avg_latency_ms ?? 1;

  return (
    <div className="overview">
      <div className="overview__section">
        <div className="overview__heading">Highest Latency</div>
        {sorted.slice(0, 4).map((e, i) => (
          <div key={i} className="overview__row">
            <div className="overview__edge">
              <span className="overview__svc">{e.source}</span>
              <span className="overview__arrow">→</span>
              <span className="overview__svc">{e.target}</span>
            </div>
            <div className="overview__bar-wrap">
              <div
                className="overview__bar"
                style={{ width: `${(e.avg_latency_ms / maxLatency) * 100}%` }}
              />
            </div>
            <span className="overview__num">{e.avg_latency_ms.toFixed(0)}ms</span>
          </div>
        ))}
      </div>

      {errorEdges.length > 0 && (
        <div className="overview__section">
          <div className="overview__heading">Error Rates</div>
          {errorEdges.slice(0, 4).map((e, i) => (
            <div key={i} className="overview__row">
              <div className="overview__edge">
                <span className="overview__svc">{e.source}</span>
                <span className="overview__arrow">→</span>
                <span className="overview__svc">{e.target}</span>
              </div>
              <div className="overview__bar-wrap">
                <div
                  className="overview__bar overview__bar--error"
                  style={{ width: `${Math.min(100, e.error_rate * 100 * 5)}%` }}
                />
              </div>
              <span className="overview__num overview__num--error">
                {(e.error_rate * 100).toFixed(1)}%
              </span>
            </div>
          ))}
        </div>
      )}

      <div className="overview__hint">Click any edge in the graph to inspect</div>
    </div>
  );
}
