import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, ImageOff, Play } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useNavigate, useParams, useSearchParams } from 'react-router'
import { toast } from 'sonner'
import { AppShell } from '@/components/AppShell'
import { PresetPanel } from '@/components/editor/PresetPanel'
import { StatusBadge } from '@/components/StatusBadge'
import { Button } from '@/components/ui/button'
import { Card, Skeleton } from '@/components/ui/misc'
import { Slider } from '@/components/ui/slider'
import { useLowRes } from '@/hooks/useImage'
import { api, saveFile, type OutputFormat } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { cn } from '@/lib/cn'
import { errorMessage } from '@/lib/errors'
import {
  ADJUSTMENTS,
  describePipeline,
  INITIAL_EDIT,
  toPipeline,
  type AdjustKey,
  type EditState,
} from '@/lib/pipeline'

const BATCH_ADJUSTMENTS: AdjustKey[] = ['brightness', 'contrast', 'saturation', 'temperature']

function ProgressBar({ value, total }: { value: number; total: number }) {
  const pct = total ? (value / total) * 100 : 0
  return (
    <div
      className="h-2 w-full overflow-hidden rounded-full bg-surface-2"
      role="progressbar"
      aria-valuemin={0}
      aria-valuemax={total}
      aria-valuenow={value}
      aria-label="Progres batch"
    >
      <div
        className="bg-rgb h-full rounded-full transition-[width] duration-200 ease-out"
        style={{ width: `${pct}%` }}
      />
    </div>
  )
}

export function BatchNew() {
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { me } = useAuth()
  const ids = useMemo(() => (params.get('ids') ?? '').split(',').filter(Boolean), [params])
  const [state, setState] = useState<EditState>(INITIAL_EDIT)
  const [format, setFormat] = useState<OutputFormat>('jpeg')

  const images = useQueries({
    queries: ids.map((id) => ({
      queryKey: ['image', id],
      queryFn: () => api.getImage(id),
      staleTime: Infinity,
    })),
  })
  const presets = useQuery({ queryKey: ['presets'], queryFn: api.presets })
  const first = images[0]?.data
  const lowRes = useLowRes(first?.url, 200)

  const start = useMutation({
    mutationFn: () =>
      api.createBatch(ids, toPipeline(state), { output_format: format, output_quality: 90 }),
    onSuccess: (batch) => {
      void queryClient.invalidateQueries({ queryKey: ['me'] })
      navigate(`/batch/${batch.id}`, { replace: true })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const remaining =
    me && me.plan.monthly_quota !== null ? me.plan.monthly_quota - me.usage.jobs_count : null

  return (
    <AppShell>
      <div className="mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 className="text-2xl">Batch edit</h1>
          <p className="text-sm text-muted">
            Satu pipeline untuk <span className="num">{ids.length}</span> foto — 1 edit per foto
            {remaining !== null && (
              <>
                {' '}
                (sisa kuota <span className="num">{remaining}</span>)
              </>
            )}
            .
          </p>
        </div>
        <Button
          size="lg"
          onClick={() => start.mutate()}
          loading={start.isPending}
          disabled={ids.length === 0}
        >
          <Play /> Mulai batch
        </Button>
      </div>
      <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
        <Card className="space-y-6 p-4">
          <PresetPanel
            src={lowRes}
            natural={{ width: first?.width ?? 0, height: first?.height ?? 0 }}
            look={state.look}
            customPresets={presets.data?.items.filter((p) => !p.is_system) ?? []}
            onSelect={(look) => setState((s) => ({ ...s, look }))}
            layout="grid"
          />
          <div className="space-y-4 border-t border-border pt-4">
            {ADJUSTMENTS.filter((a) => BATCH_ADJUSTMENTS.includes(a.key)).map((a) => (
              <Slider
                key={a.key}
                label={a.label}
                value={state.adjust[a.key]}
                min={a.min}
                max={a.max}
                step={a.step}
                defaultValue={a.default}
                onChange={(v) => setState((s) => ({ ...s, adjust: { ...s.adjust, [a.key]: v } }))}
                onCommit={() => undefined}
              />
            ))}
          </div>
          <div className="space-y-1.5 border-t border-border pt-4">
            <span className="text-xs text-muted">Format output</span>
            <div
              className="grid grid-cols-3 gap-1 rounded-[var(--radius-control)] bg-surface-2 p-1"
              role="radiogroup"
              aria-label="Format output"
            >
              {(['jpeg', 'png', 'webp'] as const).map((f) => (
                <button
                  key={f}
                  type="button"
                  role="radio"
                  aria-checked={format === f}
                  onClick={() => setFormat(f)}
                  className={cn(
                    'rounded-md py-1 text-xs font-medium uppercase',
                    format === f ? 'bg-surface text-text shadow' : 'text-muted',
                  )}
                >
                  {f}
                </button>
              ))}
            </div>
          </div>
        </Card>
        <div className="grid grid-cols-2 content-start gap-3 sm:grid-cols-3 xl:grid-cols-4">
          {images.map((q, i) =>
            q.data ? (
              <div
                key={q.data.id}
                className="overflow-hidden rounded-[var(--radius-card)] border border-border bg-surface"
              >
                <img
                  src={q.data.url}
                  alt={q.data.filename}
                  className="aspect-[4/3] w-full bg-canvas object-contain"
                />
                <p className="truncate p-2 text-xs">{q.data.filename}</p>
              </div>
            ) : q.isError ? (
              <div
                key={ids[i]}
                className="flex aspect-[4/3] items-center justify-center rounded-[var(--radius-card)] border border-border text-muted"
              >
                <ImageOff className="size-5" />
              </div>
            ) : (
              <Skeleton key={ids[i]} className="aspect-[4/3] rounded-[var(--radius-card)]" />
            ),
          )}
        </div>
      </div>
    </AppShell>
  )
}

export function BatchView() {
  const { batchId = '' } = useParams()
  const [downloading, setDownloading] = useState(false)
  const batch = useQuery({
    queryKey: ['batch', batchId],
    queryFn: () => api.getBatch(batchId),
    refetchInterval: (q) => (q.state.data?.finished ? false : 1500),
  })
  const b = batch.data

  const downloadZip = async () => {
    setDownloading(true)
    try {
      saveFile(await api.downloadBatch(batchId), `pixelcloud-batch-${batchId.slice(0, 8)}.zip`)
    } catch (err) {
      toast.error(errorMessage(err))
    } finally {
      setDownloading(false)
    }
  }

  if (batch.isError) {
    return (
      <AppShell>
        <p className="text-accent-r">{errorMessage(batch.error)}</p>
      </AppShell>
    )
  }

  return (
    <AppShell>
      <div className="mb-6 space-y-3">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div>
            <h1 className="text-2xl">Batch</h1>
            <p className="text-sm text-muted">{b ? describePipeline(b.pipeline) : '…'}</p>
          </div>
          <div className="flex gap-2">
            <Button asChild variant="ghost">
              <Link to="/gallery">Ke gallery</Link>
            </Button>
            <Button
              onClick={downloadZip}
              loading={downloading}
              disabled={!b?.finished || b.done === 0}
            >
              <Download /> Download ZIP
            </Button>
          </div>
        </div>
        {b ? (
          <>
            <ProgressBar value={b.done + b.failed} total={b.total_jobs} />
            <p className="num text-xs text-muted">
              {b.done}/{b.total_jobs} selesai
              {b.processing > 0 && ` · ${b.processing} diproses`}
              {b.queued > 0 && ` · ${b.queued} antri`}
              {b.failed > 0 && <span className="text-accent-r"> · {b.failed} gagal</span>}
            </p>
          </>
        ) : (
          <Skeleton className="h-2 w-full" />
        )}
      </div>
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
        {b
          ? b.jobs.map((j) => (
              <div
                key={j.id}
                className="overflow-hidden rounded-[var(--radius-card)] border border-border bg-surface"
              >
                <div className="flex aspect-[4/3] items-center justify-center bg-canvas">
                  {j.result_url ? (
                    <img src={j.result_url} alt={j.filename} className="size-full object-contain" />
                  ) : (
                    <StatusBadge status={j.status} />
                  )}
                </div>
                <div className="flex items-center justify-between gap-2 p-2">
                  <span className="truncate text-xs" title={j.error ?? j.filename}>
                    {j.filename}
                  </span>
                  <StatusBadge status={j.status} />
                </div>
                {j.worker_id && (
                  <p className="num px-2 pb-2 text-[10px] text-muted">{j.worker_id}</p>
                )}
              </div>
            ))
          : Array.from({ length: 6 }, (_, i) => (
              <Skeleton key={i} className="aspect-[4/3] rounded-[var(--radius-card)]" />
            ))}
      </div>
    </AppShell>
  )
}
