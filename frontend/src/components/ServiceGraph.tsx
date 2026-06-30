import * as d3 from 'd3';
import { useEffect, useRef } from 'react';
import type { Anomaly, GraphData } from '../types';

interface NodeDatum extends d3.SimulationNodeDatum {
  id: string;
}

interface LinkDatum extends d3.SimulationLinkDatum<NodeDatum> {
  source: string | NodeDatum;
  target: string | NodeDatum;
  errorRate: number;
  avgLatencyMs: number;
  callCount: number;
}

function edgeColor(errorRate: number): string {
  if (errorRate === 0) return '#22c55e';
  if (errorRate < 0.05) return '#f59e0b';
  return '#ef4444';
}

function arrowId(errorRate: number): string {
  if (errorRate === 0) return 'url(#arrow-green)';
  if (errorRate < 0.05) return 'url(#arrow-yellow)';
  return 'url(#arrow-red)';
}

function nodeId(d: string | NodeDatum): string {
  return typeof d === 'string' ? d : d.id;
}

interface Props {
  data: GraphData | null;
  anomalies: Anomaly[];
  onEdgeClick: (source: string, target: string) => void;
  selectedEdge: { source: string; target: string } | null;
}

export function ServiceGraph({ data, anomalies, onEdgeClick, selectedEdge }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const svgRef = useRef<SVGSVGElement>(null);
  const nodePositions = useRef<Map<string, { x: number; y: number }>>(new Map());

  useEffect(() => {
    const el = svgRef.current;
    const container = containerRef.current;
    if (!el || !container || !data?.edges?.length) return;

    const width = container.clientWidth || 900;
    const height = container.clientHeight || 480;

    const anomalyEdges = new Set(anomalies.map(a => `${a.source}||${a.target}`));

    const serviceSet = new Set<string>();
    data.edges.forEach(e => {
      serviceSet.add(e.source);
      serviceSet.add(e.target);
    });

    const nodes: NodeDatum[] = Array.from(serviceSet).map(id => {
      const saved = nodePositions.current.get(id);
      return { id, x: saved?.x, y: saved?.y };
    });

    const links: LinkDatum[] = data.edges.map(e => ({
      source: e.source,
      target: e.target,
      errorRate: e.error_rate,
      avgLatencyMs: e.avg_latency_ms,
      callCount: e.call_count,
    }));

    const svg = d3.select(el);
    svg.selectAll('*').remove();
    svg.attr('width', width).attr('height', height);

    // ── Defs: arrow markers + glow filter ────────────────────────
    const defs = svg.append('defs');

    (
      [
        { id: 'arrow-green',  color: '#22c55e' },
        { id: 'arrow-yellow', color: '#f59e0b' },
        { id: 'arrow-red',    color: '#ef4444' },
      ] as const
    ).forEach(({ id, color }) => {
      defs.append('marker')
        .attr('id', id)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 8).attr('refY', 0)
        .attr('markerWidth', 6).attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-5L10,0L0,5')
        .attr('fill', color);
    });

    const filter = defs.append('filter').attr('id', 'glow');
    filter.append('feGaussianBlur').attr('stdDeviation', '3').attr('result', 'blur');
    filter.append('feComposite').attr('in', 'SourceGraphic').attr('in2', 'blur').attr('operator', 'over');

    // ── Simulation ────────────────────────────────────────────────
    const sim = d3.forceSimulation(nodes)
      .force('link', d3.forceLink<NodeDatum, LinkDatum>(links).id(d => d.id).distance(220))
      .force('charge', d3.forceManyBody().strength(-700))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collide', d3.forceCollide(90));

    // ── Visible edges ─────────────────────────────────────────────
    svg.append('g')
      .selectAll<SVGLineElement, LinkDatum>('line')
      .data(links)
      .join('line')
      .attr('stroke', d => edgeColor(d.errorRate))
      .attr('stroke-width', d => Math.max(1.5, Math.min(4, d.callCount / 80)))
      .attr('marker-end', d => arrowId(d.errorRate))
      .attr('filter', d => anomalyEdges.has(`${nodeId(d.source)}||${nodeId(d.target)}`) ? 'url(#glow)' : null)
      .attr('stroke-opacity', d => {
        if (!selectedEdge) return 1;
        return selectedEdge.source === nodeId(d.source) && selectedEdge.target === nodeId(d.target) ? 1 : 0.3;
      });

    // ── Invisible wider hit targets for easy clicking ─────────────
    svg.append('g')
      .selectAll<SVGLineElement, LinkDatum>('line')
      .data(links)
      .join('line')
      .attr('stroke', 'transparent')
      .attr('stroke-width', 20)
      .attr('cursor', 'pointer')
      .on('click', (_, d) => onEdgeClick(nodeId(d.source), nodeId(d.target)));

    // ── Edge labels ───────────────────────────────────────────────
    const labelEls = svg.append('g')
      .selectAll<SVGTextElement, LinkDatum>('text')
      .data(links)
      .join('text')
      .attr('font-size', '11px')
      .attr('font-family', 'ui-monospace, Consolas, monospace')
      .attr('fill', d => {
        if (selectedEdge && !(selectedEdge.source === nodeId(d.source) && selectedEdge.target === nodeId(d.target))) {
          return '#2e3047';
        }
        return '#525570';
      })
      .attr('text-anchor', 'middle')
      .text(d => `${(d.errorRate * 100).toFixed(1)}% err · ${d.avgLatencyMs.toFixed(0)}ms`);

    // ── Nodes ─────────────────────────────────────────────────────
    const nodeEls = svg.append('g')
      .selectAll<SVGGElement, NodeDatum>('g')
      .data(nodes)
      .join('g')
      .call(
        d3.drag<SVGGElement, NodeDatum>()
          .on('start', (event, d) => {
            if (!event.active) sim.alphaTarget(0.3).restart();
            d.fx = d.x; d.fy = d.y;
          })
          .on('drag', (event, d) => { d.fx = event.x; d.fy = event.y; })
          .on('end', (event, d) => {
            if (!event.active) sim.alphaTarget(0);
            d.fx = null; d.fy = null;
          }),
      );

    nodeEls.append('rect')
      .attr('x', -76).attr('y', -22)
      .attr('width', 152).attr('height', 44)
      .attr('rx', 6)
      .attr('fill', '#1a1d27')
      .attr('stroke', '#252836')
      .attr('stroke-width', 1.5);

    nodeEls.append('text')
      .text(d => d.id)
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'central')
      .attr('font-size', '13px')
      .attr('font-weight', '500')
      .attr('font-family', 'system-ui, -apple-system, sans-serif')
      .attr('fill', '#c0c3d8');

    // ── Tick ──────────────────────────────────────────────────────
    sim.on('tick', () => {
      const x1 = (d: LinkDatum) => (d.source as NodeDatum).x ?? 0;
      const y1 = (d: LinkDatum) => (d.source as NodeDatum).y ?? 0;
      const x2 = (d: LinkDatum) => (d.target as NodeDatum).x ?? 0;
      const y2 = (d: LinkDatum) => (d.target as NodeDatum).y ?? 0;

      svg.selectAll<SVGLineElement, LinkDatum>('line')
        .attr('x1', x1).attr('y1', y1).attr('x2', x2).attr('y2', y2);

      labelEls
        .attr('x', d => (x1(d) + x2(d)) / 2)
        .attr('y', d => (y1(d) + y2(d)) / 2 - 12);

      nodeEls.attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
    });

    return () => {
      sim.stop();
      nodes.forEach(n => {
        if (n.x != null && n.y != null) {
          nodePositions.current.set(n.id, { x: n.x, y: n.y });
        }
      });
    };
  }, [data, anomalies, selectedEdge, onEdgeClick]);

  return (
    <div ref={containerRef} className="graph-container">
      {!data?.edges?.length ? (
        <div className="graph-empty">Waiting for trace data...</div>
      ) : (
        <>
          <svg ref={svgRef} />
          <div className="graph-legend">
            <span className="legend-item"><span className="legend-dot" style={{ background: '#22c55e' }} />Healthy</span>
            <span className="legend-item"><span className="legend-dot" style={{ background: '#f59e0b' }} />&lt;5% errors</span>
            <span className="legend-item"><span className="legend-dot" style={{ background: '#ef4444' }} />&gt;5% errors</span>
            <span className="legend-hint">Click any edge to inspect</span>
          </div>
        </>
      )}
    </div>
  );
}
