import { Link } from 'react-router'
import { LogoMark } from '@/components/Logo'
import { Button } from '@/components/ui/button'

export function NotFound() {
  return (
    <div className="pixel-grid flex min-h-dvh flex-col items-center justify-center gap-4 px-4 text-center">
      <LogoMark className="h-10 w-24" />
      <h1 className="text-3xl">404</h1>
      <p className="text-muted">Halaman ini tidak ada.</p>
      <Button asChild variant="secondary">
        <Link to="/">Kembali ke beranda</Link>
      </Button>
    </div>
  )
}
