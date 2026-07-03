import { Link, NavLink, Outlet, useNavigate } from 'react-router-dom'
import { Server, Users, Settings as SettingsIcon, ScrollText, LogOut } from 'lucide-react'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { ThemeToggle } from './theme-toggle'
import { LanguageToggle } from './language-toggle'
import { DisgustingLogo } from './logo'
import { useAuth } from '@/auth/auth-context'
import { useT } from '@/i18n/i18n'
import { APP_VERSION } from '@/version'

const bottomNav = [
  { to: '/', labelKey: 'nav.servers', icon: Server, end: true },
  { to: '/clients', labelKey: 'nav.clients', icon: Users, end: false },
  { to: '/logs', labelKey: 'nav.logs', icon: ScrollText, end: false },
  { to: '/settings', labelKey: 'nav.settings', icon: SettingsIcon, end: false },
]

export function AppLayout() {
  const { logout } = useAuth()
  const navigate = useNavigate()
  const t = useT()

  async function handleLogout() {
    await logout()
    navigate('/login', { replace: true })
  }

  return (
    <div className="min-h-screen bg-background">
      {/* Top bar: title left, theme + sign out (icons only) right */}
      <header className="flex h-16 items-center justify-between px-4 sm:px-6">
        <Link to="/" className="flex items-center gap-3">
          <DisgustingLogo className="h-11 w-11" />
          <span className="flex flex-col leading-none">
            <span className="text-lg font-semibold tracking-tight">Absolutely Disgusting Panel</span>
            <span className="mt-0.5 text-[11px] font-medium text-muted-foreground/70">v{APP_VERSION}</span>
          </span>
        </Link>
        <div className="flex items-center gap-1">
          <LanguageToggle />
          <ThemeToggle />
          <Button
            variant="ghost"
            size="icon"
            onClick={handleLogout}
            aria-label={t('app.signOut')}
            title={t('app.signOut')}
          >
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
            aria-label={t(item.labelKey)}
            title={t(item.labelKey)}
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

      {/* corner easter egg */}
      <span className="pointer-events-none fixed bottom-3 right-4 z-40 select-none text-[5px] font-medium tracking-wider text-muted-foreground/15">
        ВЛАД - ЛОХ
      </span>
    </div>
  )
}
