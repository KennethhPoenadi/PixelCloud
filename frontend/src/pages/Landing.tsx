import { useQuery } from '@tanstack/react-query'
import { Check, Layers, SlidersHorizontal, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { Link } from 'react-router'
import { FilteredImage } from '@/components/FilteredImage'
import { Logo } from '@/components/Logo'
import { Button } from '@/components/ui/button'
import { useElementSize } from '@/hooks/useImage'
import { api, type Plan } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { cn } from '@/lib/cn'
import { formatIDR } from '@/lib/format'
import { PRESET_OPS } from '@/lib/presets'

const DEMO = { src: '/demo.jpg', width: 1200, height: 800 }
const DEMO_OPS = [...PRESET_OPS.vintage, { op: 'brightness' as const, value: 1.08 }]

function BeforeAfter() {
  const [ref, size] = useElementSize<HTMLDivElement>()
  const [pos, setPos] = useState(55)
  const height = Math.round((size.width * DEMO.height) / DEMO.width)
  return (
    <div
      ref={ref}
      className="relative w-full overflow-hidden rounded-[var(--radius-card)] border border-border shadow-2xl"
      style={{ height }}
    >
      {size.width > 0 && (
        <>
          <FilteredImage
            src={DEMO.src}
            alt="Foto setelah filter Vintage"
            ops={DEMO_OPS}
            width={size.width}
            height={height}
            naturalWidth={DEMO.width}
          />
          <div
            className="absolute inset-0 overflow-hidden"
            style={{ clipPath: `inset(0 ${100 - pos}% 0 0)` }}
          >
            <img
              src={DEMO.src}
              alt="Foto asli"
              className="size-full object-cover"
              draggable={false}
            />
          </div>
          <div
            className="pointer-events-none absolute inset-y-0 w-0.5 bg-white/90"
            style={{ left: `${pos}%` }}
          />
          <span className="absolute top-3 left-3 rounded-full bg-black/50 px-2 py-0.5 text-xs text-white">
            Before
          </span>
          <span className="absolute top-3 right-3 rounded-full bg-black/50 px-2 py-0.5 text-xs text-white">
            After · Vintage
          </span>
          <input
            type="range"
            min={0}
            max={100}
            value={pos}
            onChange={(e) => setPos(Number(e.target.value))}
            aria-label="Geser untuk membandingkan sebelum dan sesudah"
            className="absolute inset-0 size-full cursor-ew-resize opacity-0"
          />
        </>
      )}
    </div>
  )
}

const FEATURES = [
  {
    icon: Sparkles,
    title: 'Preset sekali klik',
    body: 'Vintage, Noir, Warm, Fade dan lainnya — pratinjau langsung dari fotomu sendiri.',
  },
  {
    icon: SlidersHorizontal,
    title: 'Adjustment presisi',
    body: 'Brightness, contrast, temperature, blur, sharpen, vignette. Simpan kombinasi jadi preset.',
  },
  {
    icon: Layers,
    title: 'Batch sekaligus',
    body: 'Satu pipeline untuk puluhan foto, diproses paralel di server, unduh sebagai ZIP.',
  },
]

function PlanCard({ plan, highlight }: { plan: Plan; highlight: boolean }) {
  const perks = [
    plan.monthly_quota === null ? 'Edit tanpa batas' : `${plan.monthly_quota} edit / bulan`,
    `File hingga ${plan.max_file_mb} MB`,
    `Resolusi hingga ${plan.max_resolution} px`,
    `Batch ${plan.max_batch_size} foto`,
    plan.watermark ? 'Dengan watermark' : 'Tanpa watermark',
    ...(plan.api_access ? ['Akses API'] : []),
  ]
  return (
    <div
      className={cn(
        'relative flex flex-col rounded-[var(--radius-card)] border bg-surface p-6',
        highlight ? 'border-accent shadow-lg shadow-accent/10' : 'border-border',
      )}
    >
      {highlight && (
        <span className="absolute -top-3 left-6 rounded-full bg-accent px-2 py-0.5 text-xs font-semibold text-on-accent">
          Paling populer
        </span>
      )}
      <h3 className="text-lg">{plan.name}</h3>
      <p className="mt-2 mb-4">
        <span className="num text-3xl font-semibold">
          {plan.price_idr ? formatIDR(plan.price_idr) : 'Gratis'}
        </span>
        {plan.price_idr > 0 && <span className="text-sm text-muted"> /bulan</span>}
      </p>
      <ul className="mb-6 flex-1 space-y-2 text-sm">
        {perks.map((p) => (
          <li key={p} className="flex items-center gap-2">
            <Check className="size-4 text-accent-g" aria-hidden /> {p}
          </li>
        ))}
      </ul>
      <Button asChild variant={highlight ? 'primary' : 'secondary'}>
        <Link to="/register">Mulai</Link>
      </Button>
    </div>
  )
}

export function Landing() {
  const { token } = useAuth()
  const plans = useQuery({ queryKey: ['plans'], queryFn: api.plans, staleTime: Infinity })

  return (
    <div className="min-h-dvh">
      <header className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4">
        <Logo />
        <nav className="flex items-center gap-2">
          {token ? (
            <Button asChild>
              <Link to="/gallery">Buka gallery</Link>
            </Button>
          ) : (
            <>
              <Button asChild variant="ghost">
                <Link to="/login">Masuk</Link>
              </Button>
              <Button asChild>
                <Link to="/register">Daftar</Link>
              </Button>
            </>
          )}
        </nav>
      </header>

      <section className="pixel-grid border-y border-border">
        <div className="mx-auto grid max-w-6xl items-center gap-10 px-4 py-16 lg:grid-cols-[1fr_1.2fr] lg:py-24">
          <div>
            <h1 className="text-4xl leading-tight sm:text-5xl">
              Filter foto di cloud,{' '}
              <span className="bg-rgb bg-clip-text text-transparent">setajam piksel.</span>
            </h1>
            <p className="mt-4 max-w-md text-base text-muted">
              Upload, pilih preset atau atur sendiri, lalu render resolusi penuh di server kami —
              satu foto atau ratusan sekaligus.
            </p>
            <div className="mt-8 flex gap-3">
              <Button asChild size="lg">
                <Link to={token ? '/gallery' : '/register'}>Mulai Edit</Link>
              </Button>
              <Button asChild size="lg" variant="outline">
                <a href="#pricing">Lihat harga</a>
              </Button>
            </div>
          </div>
          <BeforeAfter />
        </div>
      </section>

      <section className="mx-auto grid max-w-6xl gap-6 px-4 py-16 md:grid-cols-3">
        {FEATURES.map((f) => (
          <div
            key={f.title}
            className="rounded-[var(--radius-card)] border border-border bg-surface p-6"
          >
            <f.icon className="mb-4 size-6 text-accent" aria-hidden />
            <h2 className="mb-2 text-lg">{f.title}</h2>
            <p className="text-sm text-muted">{f.body}</p>
          </div>
        ))}
      </section>

      <section id="pricing" className="mx-auto max-w-6xl px-4 pb-20">
        <h2 className="mb-2 text-center text-3xl">Harga sederhana</h2>
        <p className="mb-10 text-center text-muted">Mulai gratis, upgrade saat butuh lebih.</p>
        <div className="grid gap-6 md:grid-cols-3">
          {plans.data?.items.map((p) => (
            <PlanCard key={p.code} plan={p} highlight={p.code === 'pro'} />
          ))}
        </div>
      </section>

      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-4 py-6 text-sm text-muted">
          <Logo />
          <span>© {new Date().getFullYear()} PixelCloud · proyek open-source</span>
        </div>
      </footer>
    </div>
  )
}
