import { ApiError, type ErrorCode } from './api'

// Human messages per error code; the server message is used as detail where helpful.
const messages: Record<ErrorCode, string> = {
  VALIDATION_ERROR: 'Ada isian yang belum valid.',
  UNAUTHORIZED: 'Sesi kamu berakhir. Silakan masuk lagi.',
  FORBIDDEN: 'Fitur ini tidak tersedia di paketmu.',
  NOT_FOUND: 'Data tidak ditemukan — mungkin sudah dihapus.',
  CONFLICT: 'Data ini sudah ada.',
  QUOTA_EXCEEDED: 'Kuota bulan ini habis. Upgrade ke Pro untuk lanjut edit.',
  FILE_TOO_LARGE: 'File terlalu besar untuk paketmu.',
  UNSUPPORTED_FORMAT: 'Format tidak didukung. Gunakan JPEG, PNG, atau WebP.',
  RATE_LIMITED: 'Terlalu banyak permintaan. Tunggu sebentar lalu coba lagi.',
  UNAVAILABLE: 'Server sedang tidak bisa dihubungi. Coba lagi sebentar.',
  INTERNAL: 'Terjadi kesalahan di server. Coba lagi.',
}

// Codes whose server message carries useful specifics (limits, field names).
const withDetail = new Set<ErrorCode>([
  'VALIDATION_ERROR',
  'FILE_TOO_LARGE',
  'FORBIDDEN',
  'CONFLICT',
])

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    const base = messages[err.code] ?? messages.INTERNAL
    return withDetail.has(err.code) ? `${base} (${err.message})` : base
  }
  return messages.INTERNAL
}
