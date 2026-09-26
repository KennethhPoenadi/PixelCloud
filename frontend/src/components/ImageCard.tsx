import { Check, Trash2 } from 'lucide-react'
import { Link } from 'react-router'
import type { Image } from '@/lib/api'
import { cn } from '@/lib/cn'
import { formatBytes } from '@/lib/format'
import { StatusBadge } from './StatusBadge'

interface ImageCardProps {
  image: Image
  selected: boolean
  onToggleSelect: () => void
  onDelete: () => void
}

export function ImageCard({ image, selected, onToggleSelect, onDelete }: ImageCardProps) {
  const thumb = image.latest_job?.result_url ?? image.url
  return (
    <div
      className={cn(
        'group relative overflow-hidden rounded-[var(--radius-card)] border bg-surface transition-colors duration-150',
        selected ? 'border-accent' : 'border-border hover:border-muted',
      )}
    >
      <Link to={`/editor/${image.id}`} className="block aspect-[4/3] bg-canvas">
        <img src={thumb} alt={image.filename} loading="lazy" className="size-full object-contain" />
      </Link>
      <button
        type="button"
        onClick={onToggleSelect}
        aria-pressed={selected}
        aria-label={selected ? `Batal pilih ${image.filename}` : `Pilih ${image.filename}`}
        className={cn(
          'absolute top-2 left-2 flex size-6 items-center justify-center rounded-md border transition-opacity',
          selected
            ? 'border-accent bg-accent text-on-accent opacity-100'
            : 'border-white/60 bg-black/40 text-transparent opacity-0 group-hover:opacity-100 focus-visible:opacity-100',
        )}
      >
        <Check className="size-4" />
      </button>
      <button
        type="button"
        onClick={onDelete}
        aria-label={`Hapus ${image.filename}`}
        className="absolute top-2 right-2 rounded-md bg-black/50 p-1.5 text-white opacity-0 transition-opacity group-hover:opacity-100 hover:bg-accent-r focus-visible:opacity-100"
      >
        <Trash2 className="size-3.5" />
      </button>
      <div className="flex items-center justify-between gap-2 p-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium" title={image.filename}>
            {image.filename}
          </p>
          <p className="num text-[11px] text-muted">
            {image.width}×{image.height} · {formatBytes(image.size_bytes)}
          </p>
        </div>
        {image.latest_job && <StatusBadge status={image.latest_job.status} />}
      </div>
    </div>
  )
}
