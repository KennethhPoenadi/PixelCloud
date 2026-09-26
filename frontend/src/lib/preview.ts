import type { BaseOp } from './presets'

// Translate base operations into a CSS filter chain. Formulas match the worker
// (which implements the CSS Filter Effects matrices), so the preview is close
// to the final render. Temperature and sharpen need SVG filters.

const TEMP_MAX_SHIFT = 0.25 // keep in sync with worker/filters/temperature.py
const SHARPEN_KERNEL_SCALE = 0.35

export type SvgFilterDef =
  | { id: string; kind: 'temperature'; red: number; blue: number }
  | { id: string; kind: 'sharpen'; k: number }

export interface PreviewStyle {
  filter: string
  svgDefs: SvgFilterDef[]
  vignette: number
  /** Screen transform = mirror (optional) applied after rotate, i.e. `scaleX(-1) rotate(Ndeg)`. */
  rotate: number
  mirror: boolean
}

/**
 * @param idPrefix unique per rendered image, for SVG filter ids
 * @param blurScale display size / natural size, so blur radii look right when scaled
 */
export function previewStyle(ops: BaseOp[], idPrefix: string, blurScale = 1): PreviewStyle {
  const parts: string[] = []
  const svgDefs: SvgFilterDef[] = []
  let vignette = 0
  let rotate = 0
  let mirror = false

  ops.forEach((op, i) => {
    switch (op.op) {
      case 'brightness':
        parts.push(`brightness(${op.value})`)
        break
      case 'contrast':
        parts.push(`contrast(${op.value})`)
        break
      case 'saturation':
        parts.push(`saturate(${op.value})`)
        break
      case 'sepia':
        parts.push(`sepia(${op.value})`)
        break
      case 'invert':
        parts.push('invert(1)')
        break
      case 'blur':
        if (op.value > 0) parts.push(`blur(${(op.value * blurScale).toFixed(2)}px)`)
        break
      case 'temperature': {
        if (op.value === 0) break
        const k = (op.value / 100) * TEMP_MAX_SHIFT
        const id = `${idPrefix}-t${i}`
        svgDefs.push({ id, kind: 'temperature', red: 1 + k, blue: 1 - k })
        parts.push(`url(#${id})`)
        break
      }
      case 'sharpen': {
        if (op.value <= 0) break
        const id = `${idPrefix}-s${i}`
        svgDefs.push({ id, kind: 'sharpen', k: op.value * SHARPEN_KERNEL_SCALE })
        parts.push(`url(#${id})`)
        break
      }
      case 'vignette':
        vignette = Math.max(vignette, op.value)
        break
      // Compose as T = Mirror^m · Rotate(r). A later rotation R(a) gives
      // R(a)·M·R(r) = M·R(r-a), and a vertical flip equals M·R(180).
      case 'rotate':
        rotate = (rotate + (mirror ? 360 - op.angle : op.angle)) % 360
        break
      case 'flip':
        mirror = !mirror
        if (op.direction === 'v') rotate = (rotate + 180) % 360
        break
      case 'resize':
        break
    }
  })

  return { filter: parts.join(' ') || 'none', svgDefs, vignette, rotate, mirror }
}

export function vignetteBackground(strength: number): string | undefined {
  if (strength <= 0) return undefined
  return `radial-gradient(ellipse at center, transparent 40%, rgba(0,0,0,${strength}) 100%)`
}

export function transformCss(style: Pick<PreviewStyle, 'rotate' | 'mirror'>): string | undefined {
  const parts: string[] = []
  if (style.mirror) parts.push('scaleX(-1)')
  if (style.rotate) parts.push(`rotate(${style.rotate}deg)`)
  return parts.length ? parts.join(' ') : undefined
}
