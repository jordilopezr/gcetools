// Single-series trend line: one measure per chart (small multiples, never a
// second axis). 2px line, recessive grid, last point marked and labelled,
// crosshair + tooltip on hover, keyboard-free but with an accessible summary.
import { useMemo, useRef, useState } from 'react'
import { locale, t } from '../lib/i18n'

export interface Point { t: Date; v: number }

interface Props {
  title: string
  points: Point[] // oldest first
  format: (v: number) => string
  height?: number
}

const W = 360 // viewBox width; the SVG scales to its container
const PAD = { top: 12, right: 14, bottom: 22, left: 46 }

const fmtDay = (d: Date) => d.toLocaleDateString(locale(), { day: 'numeric', month: 'short' })

function niceTicks(min: number, max: number, count = 3): number[] {
  if (min === max) { min -= 1; max += 1 }
  const step0 = (max - min) / count
  const mag = 10 ** Math.floor(Math.log10(step0))
  const step = [1, 2, 2.5, 5, 10].map(m => m * mag).find(s => s >= step0) ?? step0
  const out: number[] = []
  // the last tick must reach max, or the line overflows the plot
  for (let v = Math.floor(min / step) * step; ; v += step) {
    out.push(+v.toFixed(10))
    if (v >= max - step * 1e-9) break
  }
  return out
}

export default function TrendChart({ title, points, format, height = 150 }: Props) {
  const [hover, setHover] = useState<number | null>(null)
  const svgRef = useRef<SVGSVGElement>(null)
  const H = height

  const geo = useMemo(() => {
    const vals = points.map(p => p.v)
    const lo = Math.min(...vals), hi = Math.max(...vals)
    // start the axis at 0 unless that flattens the line into the top band
    const ticks = niceTicks(lo > 0 && lo > hi * 0.6 ? lo * 0.95 : 0, hi)
    const y0 = ticks[0], y1 = ticks[ticks.length - 1]
    const t0 = points[0]?.t.getTime() ?? 0, t1 = points[points.length - 1]?.t.getTime() ?? 1
    const x = (t: number) => PAD.left + (t1 === t0 ? 0.5 : (t - t0) / (t1 - t0)) * (W - PAD.left - PAD.right)
    const y = (v: number) => PAD.top + (1 - (v - y0) / (y1 - y0 || 1)) * (H - PAD.top - PAD.bottom)
    const xy = points.map(p => [x(p.t.getTime()), y(p.v)] as const)
    return { ticks, y, xy, path: xy.map(([px, py], i) => `${i ? 'L' : 'M'}${px.toFixed(1)},${py.toFixed(1)}`).join('') }
  }, [points, H])

  if (points.length < 2) return null
  const last = points.length - 1
  const prev = points[last - 1].v
  const delta = points[last].v - prev
  const hi = hover ?? last

  const onMove = (e: React.MouseEvent<SVGRectElement>) => {
    const svg = svgRef.current
    if (!svg) return
    const r = svg.getBoundingClientRect()
    const vx = ((e.clientX - r.left) / r.width) * W
    let best = 0
    geo.xy.forEach(([px], i) => { if (Math.abs(px - vx) < Math.abs(geo.xy[best][0] - vx)) best = i })
    setHover(best)
  }

  const [hx, hy] = geo.xy[hi]
  return (
    <figure className="card outlined" style={{ margin: 0, padding: '12px 14px', position: 'relative' }}
      aria-label={t('{0}: {1} el {2}', title, format(points[last].v), fmtDay(points[last].t))}>
      <figcaption className="row" style={{ gap: 8, alignItems: 'baseline' }}>
        <span className="title-s">{title}</span>
        <span className="spacer" />
        <span className="title-m">{format(points[last].v)}</span>
        {delta !== 0 && (
          <span className="body-s muted">{delta > 0 ? '▲' : '▼'} {format(Math.abs(delta))}</span>
        )}
      </figcaption>
      <svg ref={svgRef} viewBox={`0 0 ${W} ${H}`} width="100%" role="img" style={{ display: 'block', overflow: 'visible' }}>
        {geo.ticks.map(t => (
          <g key={t}>
            <line x1={PAD.left} x2={W - PAD.right} y1={geo.y(t)} y2={geo.y(t)} stroke="var(--md-outline-variant)" strokeWidth={1} opacity={0.6} />
            <text x={PAD.left - 6} y={geo.y(t)} textAnchor="end" dominantBaseline="middle" fontSize={10} fill="var(--md-on-surface-variant)">{format(t)}</text>
          </g>
        ))}
        <text x={PAD.left} y={H - 4} fontSize={10} fill="var(--md-on-surface-variant)">{fmtDay(points[0].t)}</text>
        <text x={W - PAD.right} y={H - 4} fontSize={10} textAnchor="end" fill="var(--md-on-surface-variant)">{fmtDay(points[last].t)}</text>
        <path d={geo.path} fill="none" stroke="var(--md-primary)" strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
        {hover !== null && <line x1={hx} x2={hx} y1={PAD.top} y2={H - PAD.bottom} stroke="var(--md-outline)" strokeWidth={1} strokeDasharray="3 3" />}
        <circle cx={hx} cy={hy} r={4} fill="var(--md-primary)" stroke="var(--md-surface)" strokeWidth={2} />
        <rect x={PAD.left} y={0} width={W - PAD.left - PAD.right} height={H} fill="transparent"
          onMouseMove={onMove} onMouseLeave={() => setHover(null)} style={{ cursor: 'crosshair' }} />
      </svg>
      {hover !== null && (
        <div className="body-s" role="status" style={{
          position: 'absolute', top: 40, left: `calc(${(hx / W) * 100}% ${hx / W > 0.6 ? '- 130px' : '+ 12px'})`,
          background: 'var(--md-inverse-surface)', color: 'var(--md-inverse-on-surface)', padding: '6px 10px',
          borderRadius: 8, pointerEvents: 'none', whiteSpace: 'nowrap', boxShadow: 'var(--md-elevation-2, 0 2px 6px rgba(0,0,0,.2))',
        }}>
          <div>{points[hover].t.toLocaleString(locale(), { dateStyle: 'medium', timeStyle: 'short' })}</div>
          <div className="title-s">{format(points[hover].v)}</div>
        </div>
      )}
    </figure>
  )
}
