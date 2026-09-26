import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, KeyRound, LogOut, Trash2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'
import { AppShell } from '@/components/AppShell'
import { QuotaMeter } from '@/components/QuotaMeter'
import { Button } from '@/components/ui/button'
import { Card, Field, Skeleton } from '@/components/ui/misc'
import { Modal } from '@/components/ui/modal'
import { api } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'
import { formatBytes, formatDate, formatIDR } from '@/lib/format'

function ApiKeys() {
  const queryClient = useQueryClient()
  const keys = useQuery({ queryKey: ['api-keys'], queryFn: api.apiKeys })
  const [label, setLabel] = useState('')
  const [created, setCreated] = useState<string | null>(null)

  const create = useMutation({
    mutationFn: (l: string) => api.createApiKey(l),
    onSuccess: (k) => {
      setCreated(k.key)
      setLabel('')
      void queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })
  const revoke = useMutation({
    mutationFn: (id: string) => api.revokeApiKey(id),
    onSuccess: () => {
      toast.success('API key dicabut')
      void queryClient.invalidateQueries({ queryKey: ['api-keys'] })
    },
    onError: (err) => toast.error(errorMessage(err)),
  })

  const submit = (e: FormEvent) => {
    e.preventDefault()
    create.mutate(label.trim())
  }

  return (
    <Card>
      <h2 className="mb-1 flex items-center gap-2 text-lg">
        <KeyRound className="size-4 text-accent" /> API keys
      </h2>
      <p className="mb-4 text-sm text-muted">
        Kirim header <code className="num text-text">X-API-Key</code> untuk memanggil API dari
        servermu.
      </p>
      <form onSubmit={submit} className="mb-4 flex items-end gap-2">
        <div className="flex-1">
          <Field
            label="Label"
            name="key_label"
            placeholder="mis. server produksi"
            value={label}
            maxLength={60}
            onChange={(e) => setLabel(e.target.value)}
          />
        </div>
        <Button type="submit" loading={create.isPending}>
          Buat key
        </Button>
      </form>
      {keys.isPending ? (
        <Skeleton className="h-16" />
      ) : (
        <ul className="divide-y divide-border">
          {keys.data?.items.map((k) => (
            <li key={k.id} className="flex items-center justify-between gap-3 py-2 text-sm">
              <div className="min-w-0">
                <p className="truncate font-medium">{k.label || 'Tanpa label'}</p>
                <p className="num text-xs text-muted">
                  dibuat {formatDate(k.created_at)}
                  {k.last_used_at
                    ? ` · terakhir dipakai ${formatDate(k.last_used_at)}`
                    : ' · belum dipakai'}
                </p>
              </div>
              {k.revoked_at ? (
                <span className="text-xs text-accent-r">dicabut</span>
              ) : (
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={`Cabut ${k.label ?? 'key'}`}
                  onClick={() => revoke.mutate(k.id)}
                >
                  <Trash2 />
                </Button>
              )}
            </li>
          ))}
          {keys.data?.items.length === 0 && (
            <li className="py-2 text-sm text-muted">Belum ada API key.</li>
          )}
        </ul>
      )}
      <Modal
        open={!!created}
        onOpenChange={(o) => !o && setCreated(null)}
        title="API key baru"
        description="Salin sekarang — key ini tidak akan ditampilkan lagi."
      >
        <div className="flex items-center gap-2">
          <code className="num min-w-0 flex-1 truncate rounded-md bg-surface-2 px-3 py-2 text-xs">
            {created}
          </code>
          <Button
            variant="secondary"
            size="icon"
            aria-label="Salin key"
            onClick={() => {
              void navigator.clipboard.writeText(created ?? '')
              toast.success('Disalin')
            }}
          >
            <Copy />
          </Button>
        </div>
      </Modal>
    </Card>
  )
}

export function Account() {
  const { me, signOut } = useAuth()
  const navigate = useNavigate()
  const plans = useQuery({ queryKey: ['plans'], queryFn: api.plans, staleTime: Infinity })

  if (!me) {
    return (
      <AppShell>
        <Skeleton className="h-40" />
      </AppShell>
    )
  }
  const { plan, usage, user } = me
  return (
    <AppShell>
      <div className="mx-auto max-w-3xl space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl">{user.display_name || 'Akun'}</h1>
            <p className="text-sm text-muted">{user.email}</p>
          </div>
          <Button
            variant="outline"
            onClick={() => {
              signOut()
              navigate('/login')
            }}
          >
            <LogOut /> Keluar
          </Button>
        </div>

        <Card>
          <div className="mb-4 flex items-start justify-between gap-4">
            <div>
              <p className="text-xs tracking-wider text-muted uppercase">Paket</p>
              <p className="font-display text-2xl font-semibold">{plan.name}</p>
            </div>
            <p className="num text-sm text-muted">
              {plan.price_idr ? `${formatIDR(plan.price_idr)}/bln` : 'Gratis'}
            </p>
          </div>
          <QuotaMeter me={me} className="mb-4" />
          <dl className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
            {[
              ['Maks file', `${plan.max_file_mb} MB`],
              ['Resolusi', `${plan.max_resolution} px`],
              ['Batch', `${plan.max_batch_size} foto`],
              ['Upload bulan ini', formatBytes(usage.bytes_in)],
            ].map(([k, v]) => (
              <div key={k} className="rounded-[var(--radius-control)] bg-surface-2 p-3">
                <dt className="text-xs text-muted">{k}</dt>
                <dd className="num mt-1">{v}</dd>
              </div>
            ))}
          </dl>
          {plan.watermark && (
            <p className="mt-4 text-xs text-muted">
              Hasil render paket Free diberi watermark kecil.
            </p>
          )}
        </Card>

        {plan.api_access ? (
          <ApiKeys />
        ) : (
          <Card>
            <h2 className="mb-2 text-lg">Paket lain</h2>
            <p className="mb-4 text-sm text-muted">
              Upgrade belum tersedia online — hubungi admin untuk pindah paket.
            </p>
            <div className="grid gap-3 sm:grid-cols-3">
              {plans.data?.items.map((p) => (
                <div
                  key={p.code}
                  className={`rounded-[var(--radius-control)] border p-3 text-sm ${p.code === plan.code ? 'border-accent' : 'border-border'}`}
                >
                  <p className="font-medium">{p.name}</p>
                  <p className="num text-xs text-muted">
                    {p.monthly_quota === null ? 'Unlimited' : `${p.monthly_quota} edit/bln`}
                  </p>
                </div>
              ))}
            </div>
          </Card>
        )}
      </div>
    </AppShell>
  )
}
