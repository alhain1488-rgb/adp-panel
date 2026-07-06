import { Navigate, Route, Routes } from 'react-router-dom'
import { BrowserRouter } from 'react-router-dom'
import { useAuth } from './auth/auth-context'
import { AppLayout } from './components/layout/app-layout'
import LoginPage from './pages/Login'
import ServersPage from './pages/Servers'
import ClientsPage from './pages/Clients'
import ClientDetailPage from './pages/ClientDetail'
import SettingsPage from './pages/Settings'
import LogsPage from './pages/Logs'
import SystemPage from './pages/System'

function Protected({ children }: { children: React.ReactNode }) {
  const { authenticated, loading } = useAuth()
  if (loading) {
    return (
      <div className="flex h-screen items-center justify-center text-muted-foreground">Loading…</div>
    )
  }
  if (!authenticated) return <Navigate to="/login" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<LoginPage />} />
        <Route
          element={
            <Protected>
              <AppLayout />
            </Protected>
          }
        >
          <Route path="/" element={<ServersPage />} />
          <Route path="/clients" element={<ClientsPage />} />
          <Route path="/clients/:id" element={<ClientDetailPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/logs" element={<LogsPage />} />
          <Route path="/system" element={<SystemPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  )
}
