import * as d3 from 'd3';
import { useCallback, useEffect, useRef, useState } from 'react';
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

function nid(d: string | NodeDatum): string {
  return typeof d === 'string' ? d : d.id;
}

interface Props {
  data: GraphData | null;
  anomalies: Anomaly[];
  onEdgeClick: (source: string, target: string) => void;
  selectedEdge: { source: string; target: string } | null;
  theme?: string;
}

export function ServiceGraph({ data, anomalies, onEdgeClick, selectedEdge, theme }: Props) {
  const containerRef  = useRef<HTMLDivElement>(null);
  const svgRef        = useRef<SVGSVGElement>(null);
  const nodePositions = useRef<Map<string, { x: number; y: number }>>(new Map());
  const zoomRef       = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null);
  const nodesRef      = useRef<NodeDatum[]>([]);
  const [layoutKey, setLayoutKey] = useState(0);

  const fitToScreen = useCallback(() => {
    const el    = svgRef.current;
    const cont  = containerRef.current;
    const zoom  = zoomRef.current;
    const nodes = nodesRef.current;
    if (!el || !cont || !zoom || nodes.length === 0) return;
    const W = cont.clientWidth;
    const H = cont.clientHeight;
    const xs = nodes.map(n => n.x ?? 0);
    const ys = nodes.map(n => n.y ?? 0);
    const pad = 100;
    const minX = Math.min(...xs) - pad;
    const maxX = Math.max(...xs) + pad;
    const minY = Math.min(...ys) - pad;
    const maxY = Math.max(...ys) + pad;
    const scale = Math.min(W / (maxX - minX), H / (maxY - minY), 2.5);
    const tx = (W - scale * (minX + maxX)) / 2;
    const ty = (H - scale * (minY + maxY)) / 2;
    d3.select(el).transition().duration(500)
      .call(zoom.transform, d3.zoomIdentity.translate(tx, ty).scale(scale));
  }, []);

  const resetLayout = useCallback(() => {
    nodePositions.current.clear();
    setLayoutKey(k => k + 1);
  }, []);

  useEffect(() => {
    const el        = svgRef.current;
    const container = containerRef.current;
    if (!el || !container || !data) return;
    const hasData = (data.services?.length ?? 0) > 0 || data.edges.length > 0;
    if (!hasData) return;

    const W = container.clientWidth  || 900;
    const H = container.clientHeight || 480;

    // Theme-aware colors via CSS custom properties (inherits data-theme from ancestor)
    const style    = getComputedStyle(container);
    const cardBg   = style.getPropertyValue('--card').trim()   || '#1a1d27';
    const borderC  = style.getPropertyValue('--border').trim() || '#252836';
    const text1    = style.getPropertyValue('--text-1').trim() || '#dde1f0';
    const text3    = style.getPropertyValue('--text-3').trim() || '#525570';

    const anomalyEdgeSet = new Set(anomalies.map(a => `${a.source}||${a.target}`));
    const anomalyNodeSet = new Set(anomalies.flatMap(a => [a.source, a.target]));

    // Worst error rate per node (for border color)
    const nodeWorst = new Map<string, number>();
    data.edges.forEach(e => {
      nodeWorst.set(e.source, Math.max(nodeWorst.get(e.source) ?? 0, e.error_rate));
      nodeWorst.set(e.target, Math.max(nodeWorst.get(e.target) ?? 0, e.error_rate));
    });

    const nodeStatusColor = (id: string) => {
      if (anomalyNodeSet.has(id)) return '#ef4444';
      const er = nodeWorst.get(id) ?? 0;
      if (er >= 0.05) return '#ef4444';
      if (er > 0)     return '#f59e0b';
      return '#22c55e';
    };

    // Include every known service, not just those that appear in an edge.
    // Services that have sent spans but have no observed cross-service connections
    // still appear as isolated nodes so the operator can see them immediately.
    const serviceSet = new Set<string>(data.services ?? []);
    data.edges.forEach(e => { serviceSet.add(e.source); serviceSet.add(e.target); });

    const nodes: NodeDatum[] = Array.from(serviceSet).map(id => {
      const saved = nodePositions.current.get(id);
      // Pre-pin nodes that have saved positions so they never drift on click
      return saved
        ? { id, x: saved.x, y: saved.y, fx: saved.x, fy: saved.y }
        : { id };
    });

    const links: LinkDatum[] = data.edges.map(e => ({
      source: e.source,
      target: e.target,
      errorRate:    e.error_rate,
      avgLatencyMs: e.avg_latency_ms,
      callCount:    e.call_count,
    }));

    const svg = d3.select(el);
    svg.selectAll('*').remove();
    svg.attr('width', W).attr('height', H);

    // ── Defs ────────────────────────────────────────────────
    const defs = svg.append('defs');

    (['green', 'yellow', 'red'] as const).forEach(name => {
      const color = name === 'green' ? '#22c55e' : name === 'yellow' ? '#f59e0b' : '#ef4444';
      defs.append('marker')
        .attr('id', `arrow-${name}`)
        .attr('viewBox', '0 -5 10 10')
        .attr('refX', 9).attr('refY', 0)
        .attr('markerWidth', 6).attr('markerHeight', 6)
        .attr('orient', 'auto')
        .append('path').attr('d', 'M0,-5L10,0L0,5').attr('fill', color);
    });

    // Glow filter for anomaly edges
    const glowFilter = defs.append('filter').attr('id', 'glow')
      .attr('x', '-50%').attr('y', '-50%').attr('width', '200%').attr('height', '200%');
    glowFilter.append('feGaussianBlur').attr('stdDeviation', '4').attr('result', 'blur');
    const merge = glowFilter.append('feMerge');
    merge.append('feMergeNode').attr('in', 'blur');
    merge.append('feMergeNode').attr('in', 'SourceGraphic');

    // ── Zoom/pan ─────────────────────────────────────────────
    const zoomG = svg.append('g');
    const zoom  = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.15, 4])
      .on('zoom', e => zoomG.attr('transform', e.transform));
    svg.call(zoom).on('dblclick.zoom', null);
    zoomRef.current  = zoom;
    nodesRef.current = nodes;

    // ── Simulation ───────────────────────────────────────────
    const sim = d3.forceSimulation(nodes)
      .force('link',    d3.forceLink<NodeDatum, LinkDatum>(links).id(d => d.id).distance(210))
      .force('charge',  d3.forceManyBody().strength(-850))
      .force('center',  d3.forceCenter(W / 2, H / 2))
      .force('collide', d3.forceCollide(95));

    // ── Edge layer ───────────────────────────────────────────
    const edgeG = zoomG.append('g');

    const lineEls = edgeG.selectAll<SVGLineElement, LinkDatum>('line.vis')
      .data(links).join('line').attr('class', 'vis')
      .attr('stroke',       d => edgeColor(d.errorRate))
      .attr('stroke-width', d => Math.max(1.5, Math.min(4.5, d.callCount / 60)))
      .attr('marker-end',   d => arrowId(d.errorRate))
      .attr('filter', d => anomalyEdgeSet.has(`${nid(d.source)}||${nid(d.target)}`) ? 'url(#glow)' : null)
      .attr('stroke-opacity', d => {
        if (!selectedEdge) return 0.85;
        return (selectedEdge.source === nid(d.source) && selectedEdge.target === nid(d.target)) ? 1 : 0.18;
      });

    // Wide invisible hit targets for easy clicking
    const hitEls = edgeG.selectAll<SVGLineElement, LinkDatum>('line.hit')
      .data(links).join('line').attr('class', 'hit')
      .attr('stroke', 'transparent').attr('stroke-width', 26).attr('cursor', 'pointer')
      .on('click', (_, d) => onEdgeClick(nid(d.source), nid(d.target)));

    // ── Edge label groups (background rect + text) ───────────
    const labelG = zoomG.append('g');
    const labelGroups = labelG.selectAll<SVGGElement, LinkDatum>('g')
      .data(links).join('g')
      .attr('opacity', d => {
        if (!selectedEdge) return 1;
        return (selectedEdge.source === nid(d.source) && selectedEdge.target === nid(d.target)) ? 1 : 0.2;
      });

    labelGroups.append('rect')
      .attr('x', -46).attr('y', -9).attr('width', 92).attr('height', 17).attr('rx', 3)
      .attr('fill', cardBg).attr('fill-opacity', 0.9)
      .attr('stroke', borderC).attr('stroke-width', 0.5);

    labelGroups.append('text')
      .attr('text-anchor', 'middle').attr('dominant-baseline', 'central')
      .attr('font-size', '10px')
      .attr('font-family', 'ui-monospace, Consolas, monospace')
      .attr('font-weight', d => d.errorRate > 0 ? '600' : '400')
      .attr('fill', d => {
        if (d.errorRate >= 0.05) return '#ef4444';
        if (d.errorRate > 0)     return '#f59e0b';
        return text3;
      })
      .text(d => `${(d.errorRate * 100).toFixed(1)}% · ${d.avgLatencyMs.toFixed(0)}ms`);

    // ── Node layer ───────────────────────────────────────────
    const nodeG   = zoomG.append('g');
    const nodeEls = nodeG.selectAll<SVGGElement, NodeDatum>('g')
      .data(nodes).join('g')
      .call(d3.drag<SVGGElement, NodeDatum>()
        .on('start', (e, d) => { if (!e.active) sim.alphaTarget(0.3).restart(); d.fx = d.x; d.fy = d.y; })
        .on('drag',  (e, d) => { d.fx = e.x; d.fy = e.y; })
        // Keep node pinned after drag — don't release fx/fy or it drifts on click
        .on('end',   (e, d) => { if (!e.active) sim.alphaTarget(0); d.fx = d.x; d.fy = d.y; }));

    // Pulsing outer ring for anomaly nodes
    nodeEls.filter(d => anomalyNodeSet.has(d.id))
      .append('rect')
      .attr('class', 'anomaly-ring')
      .attr('x', -84).attr('y', -28).attr('width', 168).attr('height', 56).attr('rx', 9)
      .attr('fill', 'none').attr('stroke', '#ef4444').attr('stroke-width', 1.5);

    // Node body
    nodeEls.append('rect')
      .attr('x', -76).attr('y', -22).attr('width', 152).attr('height', 44).attr('rx', 7)
      .attr('fill', cardBg)
      .attr('stroke',       d => nodeStatusColor(d.id))
      .attr('stroke-width', 1.5);

    // Health indicator dot
    nodeEls.append('circle')
      .attr('cx', -59).attr('cy', 0).attr('r', 3.5)
      .attr('fill', d => nodeStatusColor(d.id));

    // Service name
    nodeEls.append('text')
      .text(d => d.id)
      .attr('text-anchor', 'middle').attr('dominant-baseline', 'central')
      .attr('x', 10).attr('y', 0)
      .attr('font-size', '12px').attr('font-weight', '500')
      .attr('font-family', 'system-ui, -apple-system, sans-serif')
      .attr('fill', text1);

    // Pin all nodes once the simulation settles, then auto-fit if user hasn't panned
    sim.on('end', () => {
      nodes.forEach(n => { if (n.x != null) { n.fx = n.x; n.fy = n.y; } });
      const t = d3.zoomTransform(el);
      if (t.k === 1 && t.x === 0 && t.y === 0) fitToScreen();
    });

    // ── Tick ─────────────────────────────────────────────────
    sim.on('tick', () => {
      const x1 = (d: LinkDatum) => (d.source as NodeDatum).x ?? 0;
      const y1 = (d: LinkDatum) => (d.source as NodeDatum).y ?? 0;
      const x2 = (d: LinkDatum) => (d.target as NodeDatum).x ?? 0;
      const y2 = (d: LinkDatum) => (d.target as NodeDatum).y ?? 0;

      lineEls.attr('x1', x1).attr('y1', y1).attr('x2', x2).attr('y2', y2);
      hitEls.attr('x1', x1).attr('y1', y1).attr('x2', x2).attr('y2', y2);

      labelGroups.attr('transform', d =>
        `translate(${(x1(d) + x2(d)) / 2}, ${(y1(d) + y2(d)) / 2 - 14})`
      );

      nodeEls.attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
    });

    return () => {
      sim.stop();
      nodes.forEach(n => {
        if (n.x != null && n.y != null) nodePositions.current.set(n.id, { x: n.x, y: n.y });
      });
    };
  }, [data, anomalies, selectedEdge, onEdgeClick, theme, layoutKey, fitToScreen]);

  return (
    <div ref={containerRef} className="graph-container">
      {!(data?.services?.length || data?.edges?.length) ? (
        <div className="graph-empty">Waiting for trace data...</div>
      ) : (
        <>
          <svg ref={svgRef} />
          <div className="graph-controls">
            <button className="graph-btn" onClick={fitToScreen} title="Fit all nodes to screen">Fit</button>
            <button className="graph-btn" onClick={resetLayout} title="Reset node positions">Reset</button>
          </div>
          <div className="graph-legend">
            <span className="legend-item"><span className="legend-dot" style={{ background: '#22c55e' }} />Healthy</span>
            <span className="legend-item"><span className="legend-dot" style={{ background: '#f59e0b' }} />&lt;5% err</span>
            <span className="legend-item"><span className="legend-dot" style={{ background: '#ef4444' }} />High err</span>
            <span className="legend-hint">Scroll to zoom · Drag to pan</span>
          </div>
        </>
      )}
    </div>
  );
}
