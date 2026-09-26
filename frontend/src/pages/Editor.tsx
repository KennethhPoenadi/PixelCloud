import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Eye, Minus, Plus, Redo2, SlidersHorizontal, Undo2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'
import { AdjustPanel } from '@/components/editor/AdjustPanel'
import { ExportMenu, type ExportSettings } from '@/components/editor/ExportMenu'
import { PresetPanel } from '@/components/editor/PresetPanel'
import { FilteredImage } from '@/components/FilteredImage'
import { UserMenu } from '@/components/AppShell'
import { Button } from '@/components/ui/button'
import { Field, Skeleton } from '@/components/ui/misc'
import { Modal } from '@/components/ui/modal'
import { useHistory } from '@/hooks/useHistory'
import { fitBox, useElementSize, useLowRes } from '@/hooks/useImage'
import { api, saveFile, waitForJob, type Preset } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { expandOps, INITIAL_EDIT, toPipeline, type EditState } from '@/lib/pipeline'
import { previewStyle } from '@/lib/preview'

function useNaturalSize(src: string | undefined) {
  const [size, setSize] = useState({ width: 0, height: 0 })
  useEffect(() => {
    if (!src) return
    const img = new Image()
    img.onload = () => setSize({ width: img.naturalWidth, height: img.naturalHeight })
    img.src = src
  }, [src])
  return size
}

function isTyping(e: KeyboardEvent) {
  const el = e.target as HTMLElement | null
  return !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable)
}

const ZOOMS = [0.25, 0.5, 0.75, 1, 1.5, 2, 3, 4]

export function Editor() {
  const { imageId = '' } = useParams()
  const { me } = useAuth()
  const queryClient = useQueryClient()
  const history = useHistory<EditState>(INITIAL_EDIT)
  const state = history.present
  const [showOriginal, setShowOriginal] = useState(false)
  const [zoom, setZoom] = useState(1)
  const [exportOpen, setExportOpen] = useState(false)
  const [exportSettings, setExportSettings] = useState<ExportSettings>({
    format: 'jpeg',
    quality: 90,
    maxSide: null,
  })
  const [saveOpen, setSaveOpen] = useState(false)
  const [presetName, setPresetName] = useState('')
  const [adjustOpen, setAdjustOpen] = useState(false)

  // Presigned URLs expire after a few minutes; keep the first one for the session
  // (the browser has it cached) instead of refetching and reloading the image.
  const image = useQuery({
    queryKey: ['image', imageId],
    queryFn: () => api.getImage(imageId),
    staleTime: Infinity,
  })
  const presets = useQuery({ queryKey: ['presets'], queryFn: api.presets })
  const customPresets = useMemo(
    () => presets.data?.items.filter((p) => !p.is_system) ?? [],
    [presets.data],
  )

  const src = image.data?.url
  const natural = useNaturalSize(src)
  const lowRes = useLowRes(src, 240)
  const [canvasRef, canvasSize] = useElementSize<HTMLDivElement>()

  const ops = useMemo(
    () => (showOriginal ? [] : expandOps(toPipeline(state).operations)),
    [state, showOriginal],
  )
  const sideways = previewStyle(ops, 'probe').rotate % 180 !== 0
  const shown = sideways ? { width: natural.height, height: natural.width } : natural
  const fitted = fitBox(shown.width, shown.height, canvasSize.width - 48, canvasSize.height - 48)
  const box = { width: Math.round(fitted.width * zoom), height: Math.round(fitted.height * zoom) }

  // --- rendering -----------------------------------------------------------
  const [progress, setProgress] = useState<string | null>(null)
  const render = useMutation({
    mutationFn: async () => {
      setProgress('Mengirim…')
      const created = await api.createJob(
        imageId,
        toPipeline(state, exportSettings.maxSide ?? undefined),
        { output_format: exportSettings.format, output_quality: exportSettings.quality },
      )
      setExportOpen(false)
      void queryClient.invalidateQueries({ queryKey: ['me'] })
      return waitForJob(created.id, (j) =>
        setProgress(
          j.status === 'processing'
            ? `Merender${j.worker_id ? ` di ${j.worker_id}` : ''}…`
            : 'Mengantri…',
        ),
      )
    },
    onSuccess: (j) => {
      if (j.status === 'done' && j.download_url) {
        const url = j.download_url
        toast.success('Siap diunduh', {
          description: j.worker_id ? `Dirender oleh ${j.worker_id}` : undefined,
          action: { label: 'Unduh lagi', onClick: () => saveFile(url) },
        })
        saveFile(url)
        void queryClient.invalidateQueries({ queryKey: ['images'] })
      } else {
        toast.error('Render gagal', { description: j.error ?? undefined })
      }
    },
    onError: (err) => toast.error(errorMessage(err)),
    onSettled: () => setProgress(null),
  })

  // --- presets ---------------------------------------------------------------
  const savePreset = useMutation({
    mutationFn: (name: string) => api.createPreset(name, toPipeline(state)),
    onSuccess: (p) => {
      toast.success(`Preset "${p.name}" disimpan`)
      setSaveOpen(false)
      setPresetName('')
      void queryClient.invalidateQueries({ queryKey: ['presets'] })
      history.commit({
        ...INITIAL_EDIT,
        look: { kind: 'custom', id: p.id, name: p.name, operations: p.pipeline.operations },
      })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })
  const deletePreset = useMutation({
    mutationFn: (p: Preset) => api.deletePreset(p.id),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['presets'] }),
    onError: (err) => toast.error(errorMessage(err)),
  })

  // --- keyboard shortcuts ---------------------------------------------------
  const { undo, redo } = history
  const onKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (isTyping(e)) return
      const mod = e.metaKey || e.ctrlKey
      if (mod && e.key.toLowerCase() === 'z') {
        e.preventDefault()
        if (e.shiftKey) redo()
        else undo()
      } else if (e.key === '\\') {
        setShowOriginal(true)
      } else if (!mod && e.key.toLowerCase() === 'e') {
        setExportOpen((o) => !o)
      }
    },
    [undo, redo],
  )
  useEffect(() => {
    const up = (e: KeyboardEvent) => e.key === '\\' && setShowOriginal(false)
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', up)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', up)
    }
  }, [onKeyDown])

  // Ctrl/Cmd + wheel (or trackpad pinch) zooms the canvas instead of the page.
  useEffect(() => {
    const el = canvasRef.current
    if (!el) return
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey && !e.metaKey) return
      e.preventDefault()
      setZoom((z) => {
        const i = ZOOMS.indexOf(z)
        return (
          (e.deltaY < 0 ? ZOOMS[Math.min(ZOOMS.length - 1, i + 1)] : ZOOMS[Math.max(0, i - 1)]) ?? z
        )
      })
    }
    el.addEventListener('wheel', onWheel, { passive: false })
    return () => el.removeEventListener('wheel', onWheel)
  }, [canvasRef])
  const stepZoom = (dir: 1 | -1) => {
    const i = ZOOMS.indexOf(zoom)
    setZoom(ZOOMS[Math.min(ZOOMS.length - 1, Math.max(0, i + dir))] ?? 1)
  }

  const submitPreset = (e: FormEvent) => {
    e.preventDefault()
    if (presetName.trim()) savePreset.mutate(presetName.trim())
  }

  if (image.isError) {
    return (
      <div className="flex min-h-dvh flex-col items-center justify-center gap-4">
        <p className="text-accent-r">{errorMessage(image.error)}</p>
        <Button asChild variant="secondary">
          <Link to="/gallery">Kembali ke gallery</Link>
        </Button>
      </div>
    )
  }

  const presetPanel = (layout: 'grid' | 'strip') => (
    <PresetPanel
      src={lowRes}
      natural={natural}
      look={state.look}
      customPresets={customPresets}
      onSelect={(look) => history.commit({ ...state, look })}
      onSave={() => setSaveOpen(true)}
      onDeleteCustom={(p) => deletePreset.mutate(p)}
      layout={layout}
    />
  )
  const adjustPanel = (
    <AdjustPanel
      state={state}
      onPreview={history.preview}
      onCommit={history.commit}
      onReset={() => history.commit(INITIAL_EDIT)}
    />
  )

  return (
    <div className="flex h-dvh flex-col">
      <header className="flex h-14 shrink-0 items-center gap-2 border-b border-border px-3 sm:px-4">
        <Button asChild variant="ghost" size="sm">
          <Link to="/gallery">
            <ArrowLeft /> <span className="hidden sm:inline">Gallery</span>
          </Link>
        </Button>
        <span className="min-w-0 flex-1 truncate text-sm text-muted">{image.data?.filename}</span>
        <Button
          variant="ghost"
          size="icon"
          onClick={history.undo}
          disabled={!history.canUndo}
          aria-label="Undo"
          title="Undo (Ctrl/Cmd+Z)"
        >
          <Undo2 />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          onClick={history.redo}
          disabled={!history.canRedo}
          aria-label="Redo"
          title="Redo (Ctrl/Cmd+Shift+Z)"
        >
          <Redo2 />
        </Button>
        <ExportMenu
          open={exportOpen}
          onOpenChange={setExportOpen}
          settings={exportSettings}
          onSettings={setExportSettings}
          onRender={() => !render.isPending && render.mutate()}
          progress={progress}
          watermark={!!me?.plan.watermark}
        />
        <div className="hidden sm:block">
          <UserMenu />
        </div>
      </header>

      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <aside className="hidden w-60 shrink-0 overflow-y-auto border-r border-border bg-surface p-4 lg:block">
          {presetPanel('grid')}
        </aside>

        <section className="relative min-h-0 flex-1 bg-canvas">
          <div ref={canvasRef} className="absolute inset-0 overflow-auto">
            <div
              className="flex min-h-full min-w-full items-center justify-center p-6"
              style={{ width: box.width + 48, height: box.height + 48 }}
            >
              {src && box.width > 0 ? (
                <FilteredImage
                  src={src}
                  alt={image.data?.filename ?? ''}
                  ops={ops}
                  width={box.width}
                  height={box.height}
                  naturalWidth={natural.width}
                  className="shrink-0 shadow-2xl"
                />
              ) : (
                <Skeleton className="h-2/3 w-2/3" />
              )}
            </div>
          </div>
          <div className="absolute bottom-3 left-1/2 flex -translate-x-1/2 items-center gap-1 rounded-full border border-border bg-surface/90 p-1 shadow-lg backdrop-blur">
            <Button
              variant="ghost"
              size="icon"
              className="size-8 rounded-full"
              onClick={() => stepZoom(-1)}
              aria-label="Perkecil"
            >
              <Minus />
            </Button>
            <button
              type="button"
              className="num w-12 text-center text-xs"
              onClick={() => setZoom(1)}
              title="Fit ke layar"
            >
              {Math.round(zoom * 100)}%
            </button>
            <Button
              variant="ghost"
              size="icon"
              className="size-8 rounded-full"
              onClick={() => stepZoom(1)}
              aria-label="Perbesar"
            >
              <Plus />
            </Button>
            <span className="mx-1 h-5 w-px bg-border" />
            <Button
              variant={showOriginal ? 'primary' : 'ghost'}
              size="icon"
              className="size-8 rounded-full"
              aria-label="Tahan untuk lihat original"
              title="Tahan untuk lihat original (\)"
              aria-pressed={showOriginal}
              onPointerDown={() => setShowOriginal(true)}
              onPointerUp={() => setShowOriginal(false)}
              onPointerLeave={() => setShowOriginal(false)}
            >
              <Eye />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="size-8 rounded-full lg:hidden"
              aria-label="Adjust"
              onClick={() => setAdjustOpen(true)}
            >
              <SlidersHorizontal />
            </Button>
          </div>
        </section>

        <div className="shrink-0 border-t border-border bg-surface p-3 lg:hidden">
          {presetPanel('strip')}
        </div>

        <aside className="hidden w-72 shrink-0 overflow-y-auto border-l border-border bg-surface p-4 lg:block">
          {adjustPanel}
        </aside>
      </div>

      <Modal open={adjustOpen} onOpenChange={setAdjustOpen} title="Adjust" variant="sheet">
        {adjustPanel}
      </Modal>

      <Modal
        open={saveOpen}
        onOpenChange={setSaveOpen}
        title="Simpan sebagai preset"
        description="Preset menyimpan filter & adjustment saat ini supaya bisa dipakai ulang."
      >
        <form onSubmit={submitPreset} className="space-y-4">
          <Field
            label="Nama preset"
            name="preset_name"
            value={presetName}
            maxLength={50}
            autoFocus
            onChange={(e) => setPresetName(e.target.value)}
          />
          <div className="flex justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setSaveOpen(false)}>
              Batal
            </Button>
            <Button type="submit" loading={savePreset.isPending} disabled={!presetName.trim()}>
              Simpan
            </Button>
          </div>
        </form>
      </Modal>
    </div>
  )
}
