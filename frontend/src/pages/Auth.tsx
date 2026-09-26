import { useMutation } from '@tanstack/react-query'
import { useState, type FormEvent } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router'
import { LogoMark } from '@/components/Logo'
import { Button } from '@/components/ui/button'
import { Card, Field } from '@/components/ui/misc'
import { api, ApiError } from '@/lib/api'
import { useAuth } from '@/lib/auth'
import { errorMessage } from '@/lib/errors'

type Errors = Partial<Record<'email' | 'password' | 'form', string>>

function safeNext(next: string | null): string {
  // only allow in-app paths (no open redirect)
  return next && next.startsWith('/') && !next.startsWith('//') ? next : '/gallery'
}

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const { token, signIn } = useAuth()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const next = safeNext(params.get('next'))
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [name, setName] = useState('')
  const [errors, setErrors] = useState<Errors>({})
  const isLogin = mode === 'login'

  const mutation = useMutation({
    mutationFn: () =>
      isLogin ? api.login(email, password) : api.register(email, password, name || undefined),
    onSuccess: (session) => {
      signIn(session)
      navigate(next, { replace: true })
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'UNAUTHORIZED') {
        setErrors({ password: 'Email atau password salah.' })
      } else if (err instanceof ApiError && err.code === 'CONFLICT') {
        setErrors({ email: 'Email ini sudah terdaftar. Coba masuk.' })
      } else if (err instanceof ApiError && err.code === 'VALIDATION_ERROR') {
        setErrors(
          err.message.includes('email')
            ? { email: 'Format email belum benar.' }
            : { password: 'Password 8–72 karakter.' },
        )
      } else {
        setErrors({ form: errorMessage(err) })
      }
    },
  })

  if (token) return <Navigate to={next} replace />

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const errs: Errors = {}
    if (!email.includes('@')) errs.email = 'Masukkan email yang valid.'
    if (!isLogin && password.length < 8) errs.password = 'Minimal 8 karakter.'
    if (isLogin && !password) errs.password = 'Password wajib diisi.'
    setErrors(errs)
    if (Object.keys(errs).length === 0) mutation.mutate()
  }

  return (
    <div className="pixel-grid flex min-h-dvh items-center justify-center px-4 py-10">
      <Card className="w-full max-w-sm">
        <Link to="/" className="mb-6 flex flex-col items-center gap-3">
          <LogoMark className="h-8 w-[77px]" />
          <span className="font-display text-xl font-bold">PixelCloud</span>
        </Link>
        <h1 className="mb-1 text-center text-2xl">{isLogin ? 'Masuk' : 'Buat akun'}</h1>
        <p className="mb-6 text-center text-sm text-muted">
          {isLogin ? 'Lanjutkan edit fotomu.' : 'Gratis, 50 edit per bulan.'}
        </p>
        <form onSubmit={submit} className="space-y-4" noValidate>
          {!isLogin && (
            <Field
              label="Nama (opsional)"
              name="display_name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoComplete="name"
            />
          )}
          <Field
            label="Email"
            name="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            autoComplete="email"
            error={errors.email}
            required
          />
          <Field
            label="Password"
            name="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete={isLogin ? 'current-password' : 'new-password'}
            error={errors.password}
            required
          />
          {errors.form && <p className="text-sm text-accent-r">{errors.form}</p>}
          <Button type="submit" className="w-full" size="lg" loading={mutation.isPending}>
            {isLogin ? 'Masuk' : 'Daftar'}
          </Button>
        </form>
        <p className="mt-6 text-center text-sm text-muted">
          {isLogin ? 'Belum punya akun? ' : 'Sudah punya akun? '}
          <Link
            to={`${isLogin ? '/register' : '/login'}?next=${encodeURIComponent(next)}`}
            className="font-medium text-accent hover:underline"
          >
            {isLogin ? 'Daftar' : 'Masuk'}
          </Link>
        </p>
      </Card>
    </div>
  )
}
