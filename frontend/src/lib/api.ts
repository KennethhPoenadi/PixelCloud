import type { components } from './schema'

type Schemas = components['schemas']
export type Plan = Schemas['Plan']
export type User = Schemas['User']
export type Me = Schemas['Me']
export type Session = Schemas['Session']
export type Image = Schemas['Image']
export type Preset = Schemas['Preset']
export type Pipeline = Schemas['Pipeline']
export type Operation = Schemas['Operation']
export type Job = Schemas['Job']
export type JobStatus = Schemas['JobStatus']
export type Batch = Schemas['Batch']
export type BatchJob = Schemas['BatchJob']
export type OutputFormat = Schemas['OutputFormat']
export type APIKey = Schemas['APIKey']
export type CreatedAPIKey = Schemas['CreatedAPIKey']
export type ErrorCode = Schemas['ErrorCode']

export interface Page<T> {
  items: T[]
  next_cursor: string | null
}

const BASE = '/api/v1'
const TOKEN_KEY = 'pixelcloud.token'

export class ApiError extends Error {
  readonly code: ErrorCode
  readonly status: number

  constructor(code: ErrorCode, message: string, status: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

export const tokenStore = {
  get(): string | null {
    try {
      return localStorage.getItem(TOKEN_KEY)
    } catch {
      return null
    }
  },
  set(token: string | null) {
    try {
      if (token) localStorage.setItem(TOKEN_KEY, token)
      else localStorage.removeItem(TOKEN_KEY)
    } catch {
      /* storage unavailable (private mode): session lives only in memory */
    }
  },
}

let onUnauthorized: (() => void) | null = null
export function setUnauthorizedHandler(fn: (() => void) | null) {
  onUnauthorized = fn
}

async function toApiError(res: Response): Promise<ApiError> {
  try {
    const body = (await res.json()) as Schemas['ErrorBody']
    return new ApiError(body.error.code, body.error.message, res.status)
  } catch {
    const code: ErrorCode = res.status === 413 ? 'FILE_TOO_LARGE' : 'INTERNAL'
    return new ApiError(code, res.statusText || 'Request failed', res.status)
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  const token = tokenStore.get()
  if (token) headers.Authorization = `Bearer ${token}`
  let payload: BodyInit | undefined
  if (body instanceof FormData) {
    payload = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    payload = JSON.stringify(body)
  }

  let res: Response
  try {
    res = await fetch(BASE + path, { method, headers, body: payload })
  } catch {
    throw new ApiError('UNAVAILABLE', 'Network error', 0)
  }
  if (res.status === 401 && token) onUnauthorized?.()
  if (!res.ok) throw await toApiError(res)
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

async function download(path: string): Promise<Blob> {
  const token = tokenStore.get()
  const res = await fetch(BASE + path, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  })
  if (!res.ok) throw await toApiError(res)
  return res.blob()
}

export interface OutputOptions {
  output_format: OutputFormat
  output_quality: number
}

export const api = {
  plans: () => request<{ items: Plan[] }>('GET', '/plans'),
  register: (email: string, password: string, display_name?: string) =>
    request<Session>('POST', '/auth/register', { email, password, display_name }),
  login: (email: string, password: string) =>
    request<Session>('POST', '/auth/login', { email, password }),
  me: () => request<Me>('GET', '/me'),

  listImages: (params: { cursor?: string | null; limit?: number; q?: string }) => {
    const qs = new URLSearchParams()
    if (params.cursor) qs.set('cursor', params.cursor)
    if (params.limit) qs.set('limit', String(params.limit))
    if (params.q) qs.set('q', params.q)
    return request<Page<Image>>('GET', `/images?${qs.toString()}`)
  },
  getImage: (id: string) => request<Image>('GET', `/images/${id}`),
  uploadImage: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return request<Image>('POST', '/images', form)
  },
  deleteImage: (id: string) => request<undefined>('DELETE', `/images/${id}`),

  presets: () => request<{ items: Preset[] }>('GET', '/presets'),
  createPreset: (name: string, pipeline: Pipeline) =>
    request<Preset>('POST', '/presets', { name, pipeline }),
  deletePreset: (id: string) => request<undefined>('DELETE', `/presets/${id}`),

  createJob: (image_id: string, pipeline: Pipeline, out: OutputOptions) =>
    request<Job>('POST', '/jobs', { image_id, pipeline, ...out }),
  getJob: (id: string) => request<Job>('GET', `/jobs/${id}`),

  createBatch: (image_ids: string[], pipeline: Pipeline, out: OutputOptions) =>
    request<Batch>('POST', '/batches', { image_ids, pipeline, ...out }),
  getBatch: (id: string) => request<Batch>('GET', `/batches/${id}`),
  downloadBatch: (id: string) => download(`/batches/${id}/download`),

  apiKeys: () => request<{ items: APIKey[] }>('GET', '/api-keys'),
  createApiKey: (label: string) => request<CreatedAPIKey>('POST', '/api-keys', { label }),
  revokeApiKey: (id: string) => request<undefined>('DELETE', `/api-keys/${id}`),
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

/** Poll a job once per second until it is done or failed. */
export async function waitForJob(id: string, onUpdate: (job: Job) => void): Promise<Job> {
  for (;;) {
    const job = await api.getJob(id)
    onUpdate(job)
    if (job.status === 'done' || job.status === 'failed') return job
    await sleep(1000)
  }
}

/** Save a blob or URL to disk through a temporary link. */
export function saveFile(source: Blob | string, filename?: string) {
  const url = typeof source === 'string' ? source : URL.createObjectURL(source)
  const a = document.createElement('a')
  a.href = url
  if (filename) a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  if (typeof source !== 'string') setTimeout(() => URL.revokeObjectURL(url), 1000)
}
