import { FlipHorizontal2, FlipVertical2, RotateCcw, RotateCw } from 'lucide-react'
import { ADJUSTMENTS, type AdjustKey, type EditState } from '@/lib/pipeline'
import { Button } from '../ui/button'
import { Slider } from '../ui/slider'

interface AdjustPanelProps {
  state: EditState
  onPreview: (next: EditState) => void
  onCommit: (next: EditState) => void
  onReset: () => void
}

export function AdjustPanel({ state, onPreview, onCommit, onReset }: AdjustPanelProps) {
  const withValue = (key: AdjustKey, value: number): EditState => ({
    ...state,
    adjust: { ...state.adjust, [key]: value },
  })
  const rotate = (delta: 90 | -90) =>
    onCommit({
      ...state,
      rotate: ((((state.rotate + delta) % 360) + 360) % 360) as EditState['rotate'],
    })

  return (
    <div className="space-y-5">
      <h2 className="text-[11px] font-semibold tracking-wider text-muted uppercase">Adjust</h2>
      {ADJUSTMENTS.map((a) => (
        <Slider
          key={a.key}
          label={a.label}
          value={state.adjust[a.key]}
          min={a.min}
          max={a.max}
          step={a.step}
          unit={a.unit}
          defaultValue={a.default}
          onChange={(v) => onPreview(withValue(a.key, v))}
          onCommit={(v) => onCommit(withValue(a.key, v))}
        />
      ))}
      <div className="border-t border-border pt-4">
        <h3 className="mb-2 text-[11px] font-semibold tracking-wider text-muted uppercase">
          Transform
        </h3>
        <div className="grid grid-cols-4 gap-1">
          <Button
            variant="secondary"
            size="icon"
            aria-label="Putar kiri"
            title="Putar kiri"
            onClick={() => rotate(-90)}
          >
            <RotateCcw />
          </Button>
          <Button
            variant="secondary"
            size="icon"
            aria-label="Putar kanan"
            title="Putar kanan"
            onClick={() => rotate(90)}
          >
            <RotateCw />
          </Button>
          <Button
            variant={state.flipH ? 'primary' : 'secondary'}
            size="icon"
            aria-label="Flip horizontal"
            aria-pressed={state.flipH}
            title="Flip horizontal"
            onClick={() => onCommit({ ...state, flipH: !state.flipH })}
          >
            <FlipHorizontal2 />
          </Button>
          <Button
            variant={state.flipV ? 'primary' : 'secondary'}
            size="icon"
            aria-label="Flip vertikal"
            aria-pressed={state.flipV}
            title="Flip vertikal"
            onClick={() => onCommit({ ...state, flipV: !state.flipV })}
          >
            <FlipVertical2 />
          </Button>
        </div>
      </div>
      <Button variant="outline" className="w-full" onClick={onReset}>
        Reset all
      </Button>
    </div>
  )
}
