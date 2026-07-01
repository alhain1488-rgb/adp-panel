import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Server, Users, Settings as SettingsIcon, ScrollText, LogOut, ShieldCheck } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from './theme-toggle'
import { useAuth } from '@/auth/auth-context'

const bottomNav = [
  { to: '/', label: 'Servers', icon: Server, end: true },
  { to: '/clients', label: 'Clients', icon: Users, end: false },
  { to: '/logs', label: 'Logs', icon: ScrollText, end: false },
  { to: '/settings', label: 'Settings', icon: SettingsIcon, end: false },
]

export function AppLayout() {
  const { logout } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Top bar: title left, theme + sign out (icons only) right */}
      <header className="flex h-16 items-center justify-between px-4 sm:px-6">
        <Link to="/" className="flex items-center gap-2">
          <ShieldCheck className="h-6 w-6 text-primary" />
          <span className="text-lg font-semibold tracking-tight">Absolutely Disgusting Panel</span>
        </Link>
        <div className="flex items-center gap-1">
          <ThemeToggle />
          <Button variant="ghost" size="icon" onClick={handleLogout} aria-label="Sign out" title="Sign out">
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
      </header>

      <main className="px-4 pb-24 pt-2 sm:px-6">
        <Outlet />
      </main>

      {/* Bottom-left floating nav: icon-only buttons */}
      <nav className="fixed bottom-4 left-4 z-40 flex items-center gap-2 rounded-full border bg-card/90 p-1.5 shadow-lg backdrop-blur">
        {bottomNav.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            aria-label={item.label}
            title={item.label}
            className={({ isActive }) =>
              cn(
                'flex h-10 w-10 items-center justify-center rounded-full transition-colors',
                isActive
                  ? 'bg-primary text-primary-foreground'
                  : 'text-muted-foreground hover:bg-accent hover:text-foreground',
              )
            }
          >
            <item.icon className="h-5 w-5" />
          </NavLink>
        ))}
      </nav>
    </div>
  )
}
