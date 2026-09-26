import { useEffect, useRef, useState, type RefObject } from 'react'

/**
 * Downscale an image to at most `maxSide` px once, returning a data URL. Used
 * for preset thumbnails so nine filtered previews stay cheap. Falls back to the
 * original URL if the canvas cannot be read.
 */
export function useLowRes(src: string | undefined, maxSide: number): string | undefined {
  const [low, setLow] = useState<string | undefined>()
  useEffect(() => {
    if (!src) return
    let cancelled = false
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      if (cancelled) return
      const scale = Math.min(1, maxSide / Math.max(img.naturalWidth, img.naturalHeight))
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(img.naturalWidth * scale))
      canvas.height = Math.max(1, Math.round(img.naturalHeight * scale))
      try {
        canvas.getContext('2d')?.drawImage(img, 0, 0, canvas.width, canvas.height)
        setLow(canvas.toDataURL('image/jpeg', 0.85))
      } catch {
        setLow(src)
      }
    }
    img.onerror = () => !cancelled && setLow(src)
    img.src = src
    return () => {
      cancelled = true
    }
  }, [src, maxSide])
  return low
}

/** Observe an element's content-box size. */
export function useElementSize<T extends HTMLElement>(): [
  RefObject<T | null>,
  { width: number; height: number },
] {
  const ref = useRef<T>(null)
  const [size, setSize] = useState({ width: 0, height: 0 })
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => {
      if (!entry) return
      const { width, height } = entry.contentRect
      setSize({ width: Math.floor(width), height: Math.floor(height) })
    })
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, size]
}

/** Fit a w×h box inside a container, never upscaling beyond `maxScale`. */
export function fitBox(w: number, h: number, boxW: number, boxH: number, maxScale = 1) {
  if (w <= 0 || h <= 0 || boxW <= 0 || boxH <= 0) return { width: 0, height: 0 }
  const scale = Math.min(boxW / w, boxH / h, maxScale)
  return { width: Math.round(w * scale), height: Math.round(h * scale) }
}
