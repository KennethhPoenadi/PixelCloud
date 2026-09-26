import { Check, Clock, Loader2, X } from 'lucide-react'
import type { JobStatus } from '@/lib/api'
import { cn } from '@/lib/cn'

const styles: Record<JobStatus, { label: string; className: string; icon: typeof Check }> = {
  queued: { label: 'Antri', className: 'bg-surface-2 text-muted', icon: Clock },
  processing: { label: 'Diproses', className: 'bg-accent/15 text-accent', icon: Loader2 },
  done: { label: 'Selesai', className: 'bg-accent-g/15 text-accent-g', icon: Check },
  failed: { label: 'Gagal', className: 'bg-accent-r/15 text-accent-r', icon: X },
}

export function StatusBadge({ status, className }: { status: JobStatus; className?: string }) {
  const s = styles[status]
  const Icon = s.icon
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium',
        s.className,
        className,
      )}
    >
      <Icon className={cn('size-3', status === 'processing' && 'animate-spin')} aria-hidden />
      {s.label}
    </span>
  )
}
