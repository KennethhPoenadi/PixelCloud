import * as Popover from '@radix-ui/react-popover'
import { ChevronDown, Download } from 'lucide-react'
import type { OutputFormat } from '@/lib/api'
import { cn } from '@/lib/cn'
import { Button } from '../ui/button'
import { Slider } from '../ui/slider'

export interface ExportSettings {
  format: OutputFormat
  quality: number
  maxSide: number | null
}

const FORMATS: { value: OutputFormat; label: string }[] = [
  { value: 'jpeg', label: 'JPEG' },
  { value: 'png', label: 'PNG' },
  { value: 'webp', label: 'WebP' },
]
const SIZES: { value: number | null; label: string }[] = [
  { value: null, label: 'Asli' },
  { value: 4096, label: '4096' },
  { value: 2048, label: '2048' },
  { value: 1080, label: '1080' },
]

interface ExportMenuProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: ExportSettings
  onSettings: (s: ExportSettings) => void
  onRender: () => void
  /** Human progress text while a render is running, e.g. "Merender…". */
  progress: string | null
  watermark: boolean
}

function Segmented<T>({
  label,
  options,
  value,
  onChange,
}: {
  label: string
  options: { value: T; label: string }[]
  value: T
  onChange: (v: T) => void
}) {
  return (
    <div className="space-y-1.5">
      <span className="text-xs text-muted">{label}</span>
      <div
        className="grid auto-cols-fr grid-flow-col gap-1 rounded-[var(--radius-control)] bg-surface-2 p-1"
        role="radiogroup"
        aria-label={label}
      >
        {options.map((o) => (
          <button
            key={o.label}
            type="button"
            role="radio"
            aria-checked={o.value === value}
            onClick={() => onChange(o.value)}
            className={cn(
              'rounded-md py-1 text-xs font-medium transition-colors duration-150',
              o.value === value ? 'bg-surface text-text shadow' : 'text-muted hover:text-text',
            )}
          >
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}

export function ExportMenu({
  open,
  onOpenChange,
  settings,
  onSettings,
  onRender,
  progress,
  watermark,
}: ExportMenuProps) {
  const busy = progress !== null
  return (
    <Popover.Root open={open} onOpenChange={onOpenChange}>
      <Popover.Trigger asChild>
        <Button loading={busy} aria-keyshortcuts="E">
          {busy ? progress : 'Export'} {!busy && <ChevronDown />}
        </Button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={8}
          className="z-50 w-72 space-y-4 rounded-[var(--radius-card)] border border-border bg-surface p-4 shadow-xl"
        >
          <Segmented
            label="Format"
            options={FORMATS}
            value={settings.format}
            onChange={(format) => onSettings({ ...settings, format })}
          />
          {settings.format !== 'png' && (
            <Slider
              label="Quality"
              value={settings.quality}
              min={1}
              max={100}
              step={1}
              defaultValue={90}
              onChange={(quality) => onSettings({ ...settings, quality })}
              onCommit={(quality) => onSettings({ ...settings, quality })}
            />
          )}
          <Segmented
            label="Sisi terpanjang (px)"
            options={SIZES}
            value={settings.maxSide}
            onChange={(maxSide) => onSettings({ ...settings, maxSide })}
          />
          {watermark && (
            <p className="text-xs text-muted">Paket Free menambahkan watermark kecil di pojok.</p>
          )}
          <Button className="w-full" onClick={onRender} loading={busy}>
            {!busy && <Download />} {busy ? progress : 'Render & Download'}
          </Button>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
