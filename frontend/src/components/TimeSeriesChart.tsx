import type { BucketPoint } from '../types';

interface Props {
  buckets: BucketPoint[];
}

// Fixed viewBox dimensions
const VW = 380;
const VH = 110;
const ML = 46; // left margin for y-axis labels
const MR = 12;
const MT = 10;
const MB = 26; // bottom margin for x-axis labels
const CW = VW - ML - MR;
const CH = VH - MT - MB;

function niceMax(v: number): number {
  if (v <= 0) return 1;
  const mag = Math.pow(10, Math.floor(Math.log10(v)));
  return Math.ceil(v / mag) * mag;
}

function sx(t: number, tMin: number, tMax: number): number {
  if (tMax === tMin) return ML + CW / 2;
  return ML + ((t - tMin) / (tMax - tMin)) * CW;
}

function sy(v: number, vMax: number): number {
  return MT + (1 - v / vMax) * CH;
}

function toLinePath(pts: [number, number][]): string {
  return pts.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
}

function toAreaPath(pts: [number, number][], bottom: number): string {
  if (pts.length === 0) return '';
  const last = pts[pts.length - 1];
  return `${toLinePath(pts)} L${last[0].toFixed(1)},${bottom.toFixed(1)} L${pts[0][0].toFixed(1)},${bottom.toFixed(1)} Z`;
}

function formatTime(unix: number): string {
  const d = new Date(unix * 1000);
  const h = d.getHours().toString().padStart(2, '0');
  const m = d.getMinutes().toString().padStart(2, '0');
  const s = d.getSeconds().toString().padStart(2, '0');
  return `${h}:${m}:${s}`;
}

function MiniChart({
  label,
  color,
  values,
  times,
  formatY,
}: {
  label: string;
  color: string;
  values: number[];
  times: number[];
  formatY: (v: number) => string;
}) {
  const tMin = times[0];
  const tMax = times[times.length - 1];
  const vMax = niceMax(Math.max(...values));
  const bottom = MT + CH;

  const pts: [number, number][] = values.map((v, i) => [sx(times[i], tMin, tMax), sy(v, vMax)]);

  const yTicks = [0, 0.5, 1].map(f => ({ y: MT + (1 - f) * CH, label: formatY(f * vMax) }));
  const xTicks = times.length >= 2
    ? [times[0], times[Math.floor(times.length / 2)], times[times.length - 1]]
    : [times[0]];

  return (
    <div style={{ marginBottom: 20 }}>
      <div style={{
        fontSize: 10,
        fontFamily: 'ui-monospace, Consolas, monospace',
        color: '#6b7280',
        textTransform: 'uppercase',
        letterSpacing: '0.08em',
        marginBottom: 4,
      }}>
        {label}
      </div>
      <svg viewBox={`0 0 ${VW} ${VH}`} width="100%" style={{ display: 'block' }}>
        {/* Horizontal grid lines */}
        {yTicks.map((t, i) => (
          <line key={i} x1={ML} y1={t.y} x2={ML + CW} y2={t.y}
            stroke="#1e2133" strokeWidth={1} />
        ))}

        {/* Y-axis labels */}
        {yTicks.map((t, i) => (
          <text key={i} x={ML - 5} y={t.y} textAnchor="end" dominantBaseline="middle"
            fontSize={9} fontFamily="ui-monospace, Consolas, monospace" fill="#4b5563">
            {t.label}
          </text>
        ))}

        {/* Area fill */}
        <path d={toAreaPath(pts, bottom)} fill={color} fillOpacity={0.1} />

        {/* Line */}
        <path d={toLinePath(pts)} fill="none" stroke={color}
          strokeWidth={1.5} strokeLinejoin="round" strokeLinecap="round" />

        {/* X-axis labels */}
        {xTicks.map((t, i) => (
          <text key={i}
            x={sx(t, tMin, tMax)}
            y={VH - 5}
            textAnchor={i === 0 ? 'start' : i === xTicks.length - 1 ? 'end' : 'middle'}
            fontSize={9} fontFamily="ui-monospace, Consolas, monospace" fill="#4b5563">
            {formatTime(t)}
          </text>
        ))}

        {/* Axis borders */}
        <line x1={ML} y1={MT} x2={ML} y2={bottom} stroke="#252836" strokeWidth={1} />
        <line x1={ML} y1={bottom} x2={ML + CW} y2={bottom} stroke="#252836" strokeWidth={1} />
      </svg>
    </div>
  );
}

export function TimeSeriesChart({ buckets }: Props) {
  if (buckets.length < 2) {
    return (
      <div style={{
        padding: '24px 0',
        textAlign: 'center',
        color: '#3d4158',
        fontSize: 12,
        fontFamily: 'ui-monospace, Consolas, monospace',
      }}>
        Collecting data — check back in ~30s
      </div>
    );
  }

  const times = buckets.map(b => b.t);
  const errorRates = buckets.map(b => b.er * 100);
  const latencies = buckets.map(b => b.lat);

  return (
    <div>
      <MiniChart
        label="Error Rate (%)"
        color="#ef4444"
        values={errorRates}
        times={times}
        formatY={v => `${v.toFixed(v < 10 ? 1 : 0)}%`}
      />
      <MiniChart
        label="Avg Latency (ms)"
        color="#60a5fa"
        values={latencies}
        times={times}
        formatY={v => `${v.toFixed(0)}ms`}
      />
    </div>
  );
}
