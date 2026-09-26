import { useId } from 'react'
import { Link } from 'react-router'
import { cn } from '@/lib/cn'

// Cloud made of pixel squares with the RGB gradient (brand motif).
const ROWS = ['....##......', '...####.##..', '.##########.', '############', '.##########.']
const PIXELS = ROWS.flatMap((row, y) =>
  [...row].flatMap((c, x) => (c === '#' ? [[x, y] as const] : [])),
)

export function LogoMark({ className }: { className?: string }) {
  // unique per instance: a gradient inside a hidden SVG would not resolve for the others
  const gradientId = `pc-logo-${useId().replace(/[^a-zA-Z0-9]/g, '')}`
  return (
    <svg
      viewBox="0 0 12 5"
      className={cn('h-5 w-12', className)}
      aria-hidden
      shapeRendering="crispEdges"
    >
      <defs>
        <linearGradient id={gradientId} x1="0" x2="1" y1="0" y2="0">
          <stop offset="0" stopColor="#ff4d6d" />
          <stop offset="0.5" stopColor="#2ee59d" />
          <stop offset="1" stopColor="#3d7bff" />
        </linearGradient>
      </defs>
      {PIXELS.map(([x, y]) => (
        <rect
          key={`${x}-${y}`}
          x={x + 0.08}
          y={y + 0.08}
          width={0.84}
          height={0.84}
          fill={`url(#${gradientId})`}
        />
      ))}
    </svg>
  )
}

export function Logo({ to = '/', className }: { to?: string; className?: string }) {
  return (
    <Link
      to={to}
      className={cn('flex items-center gap-2 font-display text-base font-bold', className)}
    >
      <LogoMark />
      <span>PixelCloud</span>
    </Link>
  )
}
