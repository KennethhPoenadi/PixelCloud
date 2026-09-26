import * as SliderPrimitive from '@radix-ui/react-slider'
import { useState } from 'react'
import { cn } from '@/lib/cn'

interface SliderProps {
  label: string
  value: number
  min: number
  max: number
  step: number
  defaultValue: number
  unit?: string
  /** Called continuously while dragging (preview). */
  onChange: (value: number) => void
  /** Called once the user settles on a value (history / undo). */
  onCommit: (value: number) => void
}

function decimals(step: number) {
  const s = String(step)
  return s.includes('.') ? s.split('.')[1]!.length : 0
}

/**
 * Labelled slider: label on the left, editable mono value on the right, double
 * click resets to the default, and a blue dot marks a non-default value.
 */
export function Slider({
  label,
  value,
  min,
  max,
  step,
  defaultValue,
  unit,
  onChange,
  onCommit,
}: SliderProps) {
  const digits = decimals(step)
  // null while not editing, so the field always mirrors the current value
  const [draft, setDraft] = useState<string | null>(null)
  const text = draft ?? value.toFixed(digits)

  const changed = value !== defaultValue
  const commitText = () => {
    const n = Number(text)
    setDraft(null)
    if (text.trim() === '' || Number.isNaN(n)) return
    const clamped = Math.min(max, Math.max(min, Math.round(n / step) * step))
    onChange(clamped)
    onCommit(clamped)
  }
  const reset = () => {
    onChange(defaultValue)
    onCommit(defaultValue)
  }

  return (
    <div className="space-y-2" onDoubleClick={reset}>
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs text-muted">
          {label}
          {changed && <span className="size-1.5 rounded-full bg-accent" aria-label="diubah" />}
        </span>
        <input
          aria-label={`${label} value`}
          className="num w-16 rounded-md bg-transparent px-1 text-right text-xs text-text focus:bg-surface-2"
          value={text}
          inputMode="decimal"
          onFocus={() => setDraft(text)}
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commitText}
          onKeyDown={(e) => {
            if (e.key === 'Enter') (e.target as HTMLInputElement).blur()
          }}
          onDoubleClick={(e) => e.stopPropagation()}
        />
      </div>
      <SliderPrimitive.Root
        className="relative flex h-4 w-full touch-none items-center select-none"
        min={min}
        max={max}
        step={step}
        value={[value]}
        onValueChange={([v]) => onChange(v!)}
        onValueCommit={([v]) => onCommit(v!)}
      >
        <SliderPrimitive.Track className="relative h-1 grow rounded-full bg-surface-2">
          <SliderPrimitive.Range
            className={cn('absolute h-full rounded-full', changed ? 'bg-accent' : 'bg-border')}
          />
        </SliderPrimitive.Track>
        <SliderPrimitive.Thumb
          aria-label={label}
          aria-valuetext={`${label} ${value.toFixed(digits)}${unit ?? ''}`}
          className="block size-3.5 rounded-full border-2 border-accent bg-surface shadow transition-transform duration-150 hover:scale-110"
        />
      </SliderPrimitive.Root>
    </div>
  )
}
