import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Layers, Search, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { AppShell } from '@/components/AppShell'
import { Dropzone } from '@/components/Dropzone'
import { ImageCard } from '@/components/ImageCard'
import { LogoMark } from '@/components/Logo'
import { Button } from '@/components/ui/button'
import { Modal } from '@/components/ui/modal'
import { Skeleton } from '@/components/ui/misc'
import { useUploads } from '@/hooks/useUploads'
import { api, type Image } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function Gallery() {
  const { me } = useAuth()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const uploads = useUploads()
  const [search, setSearch] = useState('')
  const q = useDebounced(search.trim(), 300)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [toDelete, setToDelete] = useState<Image | null>(null)

  const images = useInfiniteQuery({
    queryKey: ['images', q],
    queryFn: ({ pageParam }) => api.listImages({ cursor: pageParam, limit: 24, q }),
    initialPageParam: null as string | null,
    getNextPageParam: (last) => last.next_cursor,
    // keep "processing" badges fresh while jobs run
    refetchInterval: (query) =>
      query.state.data?.pages.some((p) =>
        p.items.some((i) => i.latest_job && ['queued', 'processing'].includes(i.latest_job.status)),
      )
        ? 3000
        : false,
  })
  const items = useMemo(() => images.data?.pages.flatMap((p) => p.items) ?? [], [images.data])

  const remove = useMutation({
    mutationFn: (id: string) => api.deleteImage(id),
    onSuccess: (_, id) => {
      toast.success('Foto dihapus')
      setSelected((s) => {
        const n = new Set(s)
        n.delete(id)
        return n
      })
      void queryClient.invalidateQueries({ queryKey: ['images'] })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const toggle = (id: string) =>
    setSelected((s) => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id)
      else n.add(id)
      return n
    })

  const maxBatch = me?.plan.max_batch_size ?? 0
  const startBatch = () => {
    if (selected.size > maxBatch) {
      toast.error(`Paket ${me?.plan.name} maksimal ${maxBatch} foto per batch.`)
      return
    }
    navigate(`/batch/new?ids=${[...selected].join(',')}`)
  }

  const plan = me?.plan
  const hint = plan
    ? `JPEG, PNG, atau WebP · maks ${plan.max_file_mb} MB · ${plan.max_resolution} px`
    : undefined

  return (
    <AppShell
      center={
        <label className="relative w-full max-w-sm">
          <Search
            className="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted"
            aria-hidden
          />
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Cari nama file…"
            aria-label="Cari foto"
            className="h-9 w-full rounded-[var(--radius-control)] border border-border bg-surface-2 pr-3 pl-9 text-sm placeholder:text-muted focus:border-accent focus:outline-none"
          />
        </label>
      }
    >
      <Dropzone onFiles={(f) => uploads.mutate(f)} busy={uploads.isPending} hint={hint} />

      {selected.size > 0 && (
        <div className="sticky top-16 z-20 mt-4 flex items-center justify-between gap-3 rounded-[var(--radius-card)] border border-border bg-surface px-4 py-2 shadow-lg">
          <span className="text-sm">
            <span className="num">{selected.size}</span> foto dipilih
          </span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>
              <X /> Batal
            </Button>
            <Button size="sm" onClick={startBatch}>
              <Layers /> Batch edit
            </Button>
          </div>
        </div>
      )}

      <section className="mt-6">
        {images.isPending ? (
          <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }, (_, i) => (
              <Skeleton key={i} className="aspect-[4/3] rounded-[var(--radius-card)]" />
            ))}
          </div>
        ) : images.isError ? (
          <p className="text-accent-r">{errorMessage(images.error)}</p>
        ) : items.length === 0 ? (
          <div className="pixel-grid flex flex-col items-center gap-3 rounded-[var(--radius-card)] border border-border py-16 text-center">
            <LogoMark className="h-10 w-20 opacity-80" />
            <p className="font-display text-lg font-semibold">
              {q ? 'Tidak ada foto yang cocok' : 'Upload foto pertamamu'}
            </p>
            <p className="text-sm text-muted">
              {q ? 'Coba kata kunci lain.' : 'Tarik foto ke kotak di atas untuk mulai edit.'}
            </p>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
              {items.map((img) => (
                <ImageCard
                  key={img.id}
                  image={img}
                  selected={selected.has(img.id)}
                  onToggleSelect={() => toggle(img.id)}
                  onDelete={() => setToDelete(img)}
                />
              ))}
            </div>
            {images.hasNextPage && (
              <div className="mt-6 flex justify-center">
                <Button
                  variant="secondary"
                  loading={images.isFetchingNextPage}
                  onClick={() => images.fetchNextPage()}
                >
                  Muat lebih banyak
                </Button>
              </div>
            )}
          </>
        )}
      </section>

      <Modal
        open={!!toDelete}
        onOpenChange={(o) => !o && setToDelete(null)}
        title="Hapus foto?"
        description={
          toDelete
            ? `"${toDelete.filename}" dan semua hasil editnya akan dihapus permanen.`
            : undefined
        }
      >
        <div className="flex justify-end gap-2">
          <Button variant="ghost" onClick={() => setToDelete(null)}>
            Batal
          </Button>
          <Button
            variant="destructive"
            loading={remove.isPending}
            onClick={() =>
              toDelete && remove.mutate(toDelete.id, { onSettled: () => setToDelete(null) })
            }
          >
            Hapus
          </Button>
        </div>
      </Modal>
    </AppShell>
  )
}
