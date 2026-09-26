import type { ReactNode } from 'react'
import { Navigate, Route, Routes, useLocation } from 'react-router'
import { useAuth } from './lib/auth'
import { Account } from './pages/Account'
import { AuthPage } from './pages/Auth'
import { BatchNew, BatchView } from './pages/Batch'
import { Editor } from './pages/Editor'
import { Gallery } from './pages/Gallery'
import { Landing } from './pages/Landing'
import { NotFound } from './pages/NotFound'

function RequireAuth({ children }: { children: ReactNode }) {
  const { token } = useAuth()
  const location = useLocation()
  if (!token) {
    const next = encodeURIComponent(location.pathname + location.search)
    return <Navigate to={`/login?next=${next}`} replace />
  }
  return children
}

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/login" element={<AuthPage mode="login" />} />
      <Route path="/register" element={<AuthPage mode="register" />} />
      <Route
        path="/gallery"
        element={
          <RequireAuth>
            <Gallery />
          </RequireAuth>
        }
      />
      <Route
        path="/editor/:imageId"
        element={
          <RequireAuth>
            <Editor />
          </RequireAuth>
        }
      />
      <Route
        path="/batch/new"
        element={
          <RequireAuth>
            <BatchNew />
          </RequireAuth>
        }
      />
      <Route
        path="/batch/:batchId"
        element={
          <RequireAuth>
            <BatchView />
          </RequireAuth>
        }
      />
      <Route
        path="/account"
        element={
          <RequireAuth>
            <Account />
          </RequireAuth>
        }
      />
      <Route path="*" element={<NotFound />} />
    </Routes>
  )
}
