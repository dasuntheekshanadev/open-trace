import { useEffect, useState } from 'react';
import { fetchRootCause } from '../api/client';
import type { Anomaly, RootCauseData } from '../types';

interface Props {
  anomaly: Anomaly;
}

export function RootCausePanel({ anomaly }: Props) {
  const [data, setData] = useState<RootCauseData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    setLoading(true);
    setData(null);
    fetchRootCause(anomaly.source, anomaly.target)
      .then(setData)
      .finally(() => setLoading(false));
  }, [anomaly.source, anomaly.target]);

  return (
    <div className="rootcause">
      <div className="rootcause__heading">
        <span className="rootcause__edge">
          {anomaly.source} → {anomaly.target}
        </span>
        <span className="rootcause__subtitle">
          {data ? `${data.spans_analyzed} spans · ${data.failed_spans} failed` : ''}
        </span>
      </div>

      {loading && <div className="rootcause__loading">Loading...</div>}

      {!loading && data && data.candidates.length === 0 && (
        <div className="rootcause__empty">No discriminating attributes found yet. More traffic needed.</div>
      )}

      {!loading && data && data.candidates.length > 0 && (
        <div className="table-wrap">
          <table className="rc-table">
            <thead>
              <tr>
                <th>#</th>
                <th>Attribute</th>
                <th>Value</th>
                <th>Fail rate (with)</th>
                <th>Fail rate (without)</th>
                <th>Skew</th>
                <th>Samples</th>
              </tr>
            </thead>
            <tbody>
              {data.candidates.map((c, i) => (
                <tr key={i} className={i === 0 ? 'rc-table__top-row' : ''}>
                  <td className="rc-table__rank">{i + 1}</td>
                  <td className="rc-table__attr">{c.attribute}</td>
                  <td className="rc-table__val">{c.value}</td>
                  <td className="rc-table__rate rc-table__rate--bad">
                    {(c.failure_rate_with_value * 100).toFixed(1)}%
                  </td>
                  <td className="rc-table__rate rc-table__rate--good">
                    {(c.failure_rate_without_value * 100).toFixed(1)}%
                  </td>
                  <td className="rc-table__skew">
                    <span className="skew-bar">
                      <span
                        className="skew-bar__fill"
                        style={{ width: `${Math.min(100, c.skew * 100)}%` }}
                      />
                    </span>
                    {(c.skew * 100).toFixed(1)}%
                  </td>
                  <td className="rc-table__samples">{c.sample_count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
