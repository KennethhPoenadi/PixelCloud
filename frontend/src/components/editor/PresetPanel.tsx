import { Plus, X } from 'lucide-react'
import type { Preset } from '@/lib/api'
import { cn } from '@/lib/cn'
import { expandOps, type Look } from '@/lib/pipeline'
import { PRESET_LABELS, PRESET_OPS, SYSTEM_PRESETS, type BaseOp } from '@/lib/presets'
import { FilteredImage } from '../FilteredImage'

const THUMB = 76

interface PresetThumbProps {
  src: string | undefined
  natural: { width: number; height: number }
  ops: BaseOp[]
  label: string
  active: boolean
  onClick: () => void
  onDelete?: () => void
}

export function PresetThumb({
  src,
  natural,
  ops,
  label,
  active,
  onClick,
  onDelete,
}: PresetThumbProps) {
  // cover-fit the photo into a square thumbnail
  const scale =
    natural.width && natural.height ? THUMB / Math.min(natural.width, natural.height) : 0
  const w = Math.round(natural.width * scale)
  const h = Math.round(natural.height * scale)
  return (
    <div className="group relative">
      <button
        type="button"
        onClick={onClick}
        aria-pressed={active}
        className="flex w-full flex-col items-center gap-1.5 rounded-[var(--radius-control)] p-1 text-xs transition-colors duration-150 hover:bg-surface-2"
      >
        <span
          className={cn(
            'relative flex size-[76px] items-center justify-center overflow-hidden rounded-[var(--radius-control)] bg-canvas',
            active && 'ring-2 ring-accent ring-offset-2 ring-offset-surface',
          )}
        >
          {src && w > 0 ? (
            <FilteredImage
              src={src}
              alt=""
              ops={ops}
              width={w}
              height={h}
              naturalWidth={natural.width}
              className="shrink-0"
            />
          ) : (
            <span className="size-full animate-pulse bg-surface-2" />
          )}
        </span>
        <span className={cn('max-w-full truncate', active ? 'text-text' : 'text-muted')}>
          {label}
        </span>
      </button>
      {onDelete && (
        <button
          type="button"
          onClick={onDelete}
          aria-label={`Hapus preset ${label}`}
          className="absolute top-0 right-0 rounded-full bg-surface-2 p-0.5 text-muted opacity-0 transition-opacity group-hover:opacity-100 hover:text-accent-r focus-visible:opacity-100"
        >
          <X className="size-3" />
        </button>
      )}
    </div>
  )
}

interface PresetPanelProps {
  src: string | undefined
  natural: { width: number; height: number }
  look: Look
  customPresets: Preset[]
  onSelect: (look: Look) => void
  onSave?: () => void
  onDeleteCustom?: (preset: Preset) => void
  layout: 'grid' | 'strip'
}

export function PresetPanel({
  src,
  natural,
  look,
  customPresets,
  onSelect,
  onSave,
  onDeleteCustom,
  layout,
}: PresetPanelProps) {
  const listClass =
    layout === 'grid'
      ? 'grid grid-cols-2 gap-1'
      : 'flex gap-1 overflow-x-auto pb-1 [&>*]:w-[88px] [&>*]:shrink-0'
  return (
    <div className="space-y-4">
      <div>
        <h2 className="mb-2 text-[11px] font-semibold tracking-wider text-muted uppercase">
          Presets
        </h2>
        <div className={listClass}>
          <PresetThumb
            src={src}
            natural={natural}
            ops={[]}
            label="Original"
            active={look.kind === 'none'}
            onClick={() => onSelect({ kind: 'none' })}
          />
          {SYSTEM_PRESETS.map((name) => (
            <PresetThumb
              key={name}
              src={src}
              natural={natural}
              ops={PRESET_OPS[name]}
              label={PRESET_LABELS[name]}
              active={look.kind === 'system' && look.name === name}
              onClick={() => onSelect({ kind: 'system', name })}
            />
          ))}
        </div>
      </div>
      {(onSave || customPresets.length > 0) && (
        <div>
          <h2 className="mb-2 text-[11px] font-semibold tracking-wider text-muted uppercase">
            My presets
          </h2>
          <div className={listClass}>
            {customPresets.map((p) => (
              <PresetThumb
                key={p.id}
                src={src}
                natural={natural}
                ops={expandOps(p.pipeline.operations)}
                label={p.name}
                active={look.kind === 'custom' && look.id === p.id}
                onClick={() =>
                  onSelect({
                    kind: 'custom',
                    id: p.id,
                    name: p.name,
                    operations: p.pipeline.operations,
                  })
                }
                onDelete={onDeleteCustom ? () => onDeleteCustom(p) : undefined}
              />
            ))}
            {onSave && (
              <button
                type="button"
                onClick={onSave}
                className="flex flex-col items-center gap-1.5 rounded-[var(--radius-control)] p-1 text-xs text-muted hover:bg-surface-2 hover:text-text"
              >
                <span className="flex size-[76px] items-center justify-center rounded-[var(--radius-control)] border border-dashed border-border">
                  <Plus className="size-5" />
                </span>
                Simpan preset
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
