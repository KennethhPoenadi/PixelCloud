import * as DropdownMenu from '@radix-ui/react-dropdown-menu'
import { LogOut, Moon, Sun, User } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link, useNavigate } from 'react-router'
import { useAuth } from '@/lib/auth'
import { useTheme } from '@/lib/theme'
import { Logo } from './Logo'
import { QuotaMeter } from './QuotaMeter'

export function UserMenu() {
  const { me, signOut } = useAuth()
  const [theme, toggleTheme] = useTheme()
  const navigate = useNavigate()
  const initial = (me?.user.display_name || me?.user.email || '?').charAt(0).toUpperCase()
  const item =
    'flex cursor-pointer items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-surface-2'
  return (
    <DropdownMenu.Root>
      <DropdownMenu.Trigger
        aria-label="Menu akun"
        className="flex size-8 items-center justify-center rounded-full bg-accent font-display text-sm font-bold text-on-accent"
      >
        {initial}
      </DropdownMenu.Trigger>
      <DropdownMenu.Portal>
        <DropdownMenu.Content
          align="end"
          sideOffset={8}
          className="z-50 min-w-48 rounded-[var(--radius-card)] border border-border bg-surface p-1 shadow-xl"
        >
          {me && <div className="truncate px-2 py-1.5 text-xs text-muted">{me.user.email}</div>}
          <DropdownMenu.Item className={item} onSelect={() => navigate('/account')}>
            <User className="size-4" /> Akun & paket
          </DropdownMenu.Item>
          <DropdownMenu.Item
            className={item}
            onSelect={(e) => {
              e.preventDefault()
              toggleTheme()
            }}
          >
            {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
            {theme === 'dark' ? 'Mode terang' : 'Mode gelap'}
          </DropdownMenu.Item>
          <DropdownMenu.Separator className="my-1 h-px bg-border" />
          <DropdownMenu.Item
            className={`${item} text-accent-r`}
            onSelect={() => {
              signOut()
              navigate('/login')
            }}
          >
            <LogOut className="size-4" /> Keluar
          </DropdownMenu.Item>
        </DropdownMenu.Content>
      </DropdownMenu.Portal>
    </DropdownMenu.Root>
  )
}

export function AppShell({ children, center }: { children: ReactNode; center?: ReactNode }) {
  const { me } = useAuth()
  return (
    <div className="min-h-dvh">
      <header className="sticky top-0 z-30 border-b border-border bg-bg/90 backdrop-blur">
        <div className="mx-auto flex h-14 max-w-7xl items-center gap-4 px-4">
          <Logo to="/gallery" />
          <div className="flex min-w-0 flex-1 justify-center">{center}</div>
          {me && (
            <Link to="/account" className="hidden sm:block">
              <QuotaMeter me={me} />
            </Link>
          )}
          <UserMenu />
        </div>
      </header>
      <main className="mx-auto max-w-7xl px-4 py-6">{children}</main>
    </div>
  )
}
