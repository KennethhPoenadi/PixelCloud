import { useId, type CSSProperties } from 'react'
import type { BaseOp } from '@/lib/presets'
import { previewStyle, transformCss, vignetteBackground, type SvgFilterDef } from '@/lib/preview'
import { cn } from '@/lib/cn'

function SvgDefs({ defs }: { defs: SvgFilterDef[] }) {
  if (defs.length === 0) return null
  return (
    <svg width="0" height="0" className="absolute" aria-hidden focusable="false">
      <defs>
        {defs.map((d) =>
          d.kind === 'temperature' ? (
            <filter key={d.id} id={d.id} colorInterpolationFilters="sRGB">
              <feColorMatrix
                type="matrix"
                values={`${d.red} 0 0 0 0  0 1 0 0 0  0 0 ${d.blue} 0 0  0 0 0 1 0`}
              />
            </filter>
          ) : (
            <filter key={d.id} id={d.id} colorInterpolationFilters="sRGB">
              <feConvolveMatrix
                order="3"
                preserveAlpha="true"
                kernelMatrix={`0 ${-d.k} 0 ${-d.k} ${1 + 4 * d.k} ${-d.k} 0 ${-d.k} 0`}
              />
            </filter>
          ),
        )}
      </defs>
    </svg>
  )
}

interface FilteredImageProps {
  src: string
  alt: string
  ops: BaseOp[]
  /** Size of the box the image is fitted into (after rotation). */
  width: number
  height: number
  /** Natural width of the image, used to scale blur radii to display size. */
  naturalWidth: number
  className?: string
  style?: CSSProperties
}

/**
 * Renders an image with the pipeline applied as CSS/SVG filters. The outer box
 * is the rotated footprint; the <img> inside is sized to the unrotated shape
 * and rotated into place.
 */
export function FilteredImage({
  src,
  alt,
  ops,
  width,
  height,
  naturalWidth,
  className,
  style,
}: FilteredImageProps) {
  const rawId = useId()
  const idPrefix = `pc${rawId.replace(/[^a-zA-Z0-9]/g, '')}`
  const sideways = previewStyle(ops, idPrefix).rotate % 180 !== 0
  const imgW = sideways ? height : width
  const s = previewStyle(ops, idPrefix, naturalWidth > 0 ? imgW / naturalWidth : 1)

  return (
    <div className={cn('relative overflow-hidden', className)} style={{ width, height, ...style }}>
      <SvgDefs defs={s.svgDefs} />
      <img
        src={src}
        alt={alt}
        draggable={false}
        className="absolute top-1/2 left-1/2 max-w-none select-none"
        style={{
          width: sideways ? height : width,
          height: sideways ? width : height,
          filter: s.filter,
          transform: `translate(-50%, -50%) ${transformCss(s) ?? ''}`,
        }}
      />
      {s.vignette > 0 && (
        <div
          className="pointer-events-none absolute inset-0"
          style={{ background: vignetteBackground(s.vignette) }}
        />
      )}
    </div>
  )
}
