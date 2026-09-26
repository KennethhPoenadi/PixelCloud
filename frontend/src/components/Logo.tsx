import { Link } from 'react-router'
import { cn } from '@/lib/cn'

// Cloud made of pixel squares with the RGB gradient (brand motif).
const PIXELS: [number, number][] = [
  [3, 0],
  [4, 0],
  [1, 1],
  [2, 1],
  [3, 1],
  [4, 1],
  [5, 1],
  [0, 2],
  [1, 2],
  [2, 2],
  [3, 2],
  [4, 2],
  [5, 2],
  [6, 2],
  [0, 3],
  [1, 3],
  [2, 3],
  [3, 3],
  [4, 3],
  [5, 3],
  [6, 3],
  [7, 3],
]

export function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 8 4"
      className={cn('h-5 w-10', className)}
      aria-hidden
      shapeRendering="crispEdges"
    >
      <defs>
        <linearGradient id="pc-logo-rgb" x1="0" x2="1" y1="0" y2="0">
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
          fill="url(#pc-logo-rgb)"
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
