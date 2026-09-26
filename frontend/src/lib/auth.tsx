import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, setUnauthorizedHandler, tokenStore, type Me, type Session } from './api'

interface AuthState {
  token: string | null
  me: Me | undefined
  loadingMe: boolean
  signIn: (session: Session) => void
  signOut: () => void
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient()
  const [token, setToken] = useState<string | null>(() => tokenStore.get())

  const signOut = useCallback(() => {
    tokenStore.set(null)
    setToken(null)
    queryClient.clear()
  }, [queryClient])

  const signIn = useCallback(
    (session: Session) => {
      tokenStore.set(session.token)
      setToken(session.token)
      queryClient.clear()
    },
    [queryClient],
  )

  useEffect(() => {
    setUnauthorizedHandler(signOut)
    return () => setUnauthorizedHandler(null)
  }, [signOut])

  const meQuery = useQuery({
    queryKey: ['me'],
    queryFn: api.me,
    enabled: !!token,
    staleTime: 15_000,
  })

  const value = useMemo<AuthState>(
    () => ({ token, me: meQuery.data, loadingMe: meQuery.isPending && !!token, signIn, signOut }),
    [token, meQuery.data, meQuery.isPending, signIn, signOut],
  )
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
