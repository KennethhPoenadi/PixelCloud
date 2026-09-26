// Presigned URLs change on every API response (new signature timestamp). Reusing
// the first URL for a given object keeps <img> from re-downloading on refetch.
// URLs are refreshed well before the server-side expiry (PRESIGN_TTL, 10 min).
const MAX_AGE_MS = 5 * 60 * 1000
const cache = new Map<string, { url: string; at: number }>()

export function stableUrl(key: string, url: string): string
export function stableUrl(key: string, url: string | null | undefined): string | undefined
export function stableUrl(key: string, url: string | null | undefined): string | undefined {
  if (!url) return undefined
  const hit = cache.get(key)
  const now = Date.now()
  if (hit && now - hit.at < MAX_AGE_MS) return hit.url
  cache.set(key, { url, at: now })
  return url
}
