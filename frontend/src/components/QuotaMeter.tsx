import type { Me } from '@/lib/api'
import { cn } from '@/lib/cn'

export function QuotaMeter({ me, className }: { me: Me; className?: string }) {
  const quota = me.plan.monthly_quota
  const used = me.usage.jobs_count
  if (quota === null) {
    return (
      <span className={cn('num text-xs text-muted', className)}>
        {used} edit · <span className="text-text">Unlimited</span>
      </span>
    )
  }
  const ratio = Math.min(1, used / quota)
  const danger = ratio > 0.9
  return (
    <div className={cn('flex items-center gap-2', className)} title="Kuota edit bulan ini">
      <span className="num text-xs text-muted">
        <span className={danger ? 'text-accent-r' : 'text-text'}>{used}</span> / {quota} edit bulan
        ini
      </span>
      <div
        className="h-1.5 w-16 overflow-hidden rounded-full bg-surface-2"
        role="meter"
        aria-valuemin={0}
        aria-valuemax={quota}
        aria-valuenow={used}
        aria-label="Kuota edit bulan ini"
      >
        <div
          className={cn('h-full rounded-full transition-all', danger ? 'bg-accent-r' : 'bg-accent')}
          style={{ width: `${ratio * 100}%` }}
        />
      </div>
    </div>
  )
}
