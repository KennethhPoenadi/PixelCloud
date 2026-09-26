// Mirror of worker/worker/filters/presets.py, used only for the live preview.
// The worker's table is the source of truth; keep both in sync.

export type BaseOp =
  | {
      op:
        | 'brightness'
        | 'contrast'
        | 'saturation'
        | 'temperature'
        | 'blur'
        | 'sharpen'
        | 'vignette'
        | 'sepia'
      value: number
    }
  | { op: 'invert' }
  | { op: 'rotate'; angle: 90 | 180 | 270 }
  | { op: 'flip'; direction: 'h' | 'v' }
  | { op: 'resize'; max_width?: number; max_height?: number }

export const SYSTEM_PRESETS = [
  'grayscale',
  'sepia',
  'vintage',
  'warm',
  'cool',
  'vivid',
  'noir',
  'fade',
  'invert',
] as const
export type SystemPreset = (typeof SYSTEM_PRESETS)[number]

export const PRESET_LABELS: Record<SystemPreset, string> = {
  grayscale: 'Grayscale',
  sepia: 'Sepia',
  vintage: 'Vintage',
  warm: 'Warm',
  cool: 'Cool',
  vivid: 'Vivid',
  noir: 'Noir',
  fade: 'Fade',
  invert: 'Invert',
}

export const PRESET_OPS: Record<SystemPreset, BaseOp[]> = {
  grayscale: [{ op: 'saturation', value: 0 }],
  sepia: [{ op: 'sepia', value: 1 }],
  vintage: [
    { op: 'sepia', value: 0.6 },
    { op: 'contrast', value: 0.85 },
    { op: 'vignette', value: 0.5 },
  ],
  warm: [{ op: 'temperature', value: 35 }],
  cool: [{ op: 'temperature', value: -35 }],
  vivid: [
    { op: 'saturation', value: 1.4 },
    { op: 'contrast', value: 1.15 },
  ],
  noir: [
    { op: 'saturation', value: 0 },
    { op: 'contrast', value: 1.5 },
  ],
  fade: [
    { op: 'contrast', value: 0.9 },
    { op: 'brightness', value: 1.05 },
    { op: 'saturation', value: 0.7 },
  ],
  invert: [{ op: 'invert' }],
}

export function isSystemPreset(name: string): name is SystemPreset {
  return (SYSTEM_PRESETS as readonly string[]).includes(name)
}
