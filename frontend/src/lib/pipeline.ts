import type { Operation, Pipeline } from './api'
import { PRESET_OPS, isSystemPreset, type BaseOp } from './presets'

export type AdjustKey =
  'brightness' | 'contrast' | 'saturation' | 'temperature' | 'blur' | 'sharpen' | 'vignette'

export interface AdjustSpec {
  key: AdjustKey
  label: string
  min: number
  max: number
  step: number
  default: number
  unit?: string
}

// Ranges from the plan (§2.1); the API and worker enforce the same bounds.
export const ADJUSTMENTS: AdjustSpec[] = [
  { key: 'brightness', label: 'Brightness', min: 0, max: 2, step: 0.01, default: 1 },
  { key: 'contrast', label: 'Contrast', min: 0, max: 2, step: 0.01, default: 1 },
  { key: 'saturation', label: 'Saturation', min: 0, max: 2, step: 0.01, default: 1 },
  { key: 'temperature', label: 'Temperature', min: -100, max: 100, step: 1, default: 0 },
  { key: 'blur', label: 'Blur', min: 0, max: 20, step: 0.5, default: 0, unit: 'px' },
  { key: 'sharpen', label: 'Sharpen', min: 0, max: 3, step: 0.05, default: 0 },
  { key: 'vignette', label: 'Vignette', min: 0, max: 1, step: 0.01, default: 0 },
]

export type Adjustments = Record<AdjustKey, number>

export const DEFAULT_ADJUSTMENTS: Adjustments = Object.fromEntries(
  ADJUSTMENTS.map((a) => [a.key, a.default]),
) as Adjustments

export type Look =
  | { kind: 'none' }
  | { kind: 'system'; name: string }
  | { kind: 'custom'; id: string; name: string; operations: Operation[] }

export interface EditState {
  look: Look
  adjust: Adjustments
  rotate: 0 | 90 | 180 | 270
  flipH: boolean
  flipV: boolean
}

export const INITIAL_EDIT: EditState = {
  look: { kind: 'none' },
  adjust: DEFAULT_ADJUSTMENTS,
  rotate: 0,
  flipH: false,
  flipV: false,
}

/** Pipeline order: look (preset) → adjustments → rotate → flips → optional resize. */
export function toPipeline(state: EditState, maxSide?: number): Pipeline {
  const ops: Operation[] = []
  if (state.look.kind === 'system') ops.push({ op: 'preset', name: state.look.name as never })
  if (state.look.kind === 'custom') ops.push(...state.look.operations)
  for (const a of ADJUSTMENTS) {
    const v = state.adjust[a.key]
    if (v !== a.default) ops.push({ op: a.key, value: v })
  }
  if (state.rotate) ops.push({ op: 'rotate', angle: state.rotate })
  if (state.flipH) ops.push({ op: 'flip', direction: 'h' })
  if (state.flipV) ops.push({ op: 'flip', direction: 'v' })
  if (maxSide) ops.push({ op: 'resize', max_width: maxSide, max_height: maxSide })
  return { version: 1, operations: ops }
}

/** Expand presets into base operations (for the preview). */
export function expandOps(ops: Operation[]): BaseOp[] {
  const out: BaseOp[] = []
  for (const op of ops) {
    if (op.op === 'preset') {
      if (op.name && isSystemPreset(op.name)) out.push(...PRESET_OPS[op.name])
    } else {
      out.push(op as BaseOp)
    }
  }
  return out
}

export function isChanged(state: EditState): boolean {
  return toPipeline(state).operations.length > 0
}

export function describePipeline(p: Pipeline): string {
  if (p.operations.length === 0) return 'Tanpa filter'
  return p.operations
    .map((o) => (o.op === 'preset' ? o.name : o.op))
    .filter(Boolean)
    .join(' · ')
}
