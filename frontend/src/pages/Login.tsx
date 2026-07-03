import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { KeyRound, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { useAuth } from '@/auth/auth-context'
import { ThemeToggle } from '@/components/layout/theme-toggle'
import { LanguageToggle } from '@/components/layout/language-toggle'
import { DisgustingLogo } from '@/components/layout/logo'
import { RequestError } from '@/api/client'
import { useT } from '@/i18n/i18n'

export default function LoginPage() {
  const { login, verifyTotp } = useAuth()
  const navigate = useNavigate()
  const t = useT()

  const [step, setStep] = useState<'credentials' | 'totp'>('credentials')
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [code, setCode] = useState('')
  const [challengeId, setChallengeId] = useState<string | undefined>()
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  async function handleCredentials(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      const res = await login(username, password)
      if (res.need_2fa) {
        setChallengeId(res.challenge_id)
        setStep('totp')
      } else {
        navigate('/', { replace: true })
      }
    } catch (err) {
      setError(err instanceof RequestError ? err.message : t('login.invalidCreds'))
    } finally {
      setLoading(false)
    }
  }

  async function handleTotp(e: React.FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await verifyTotp(code, challengeId)
      navigate('/', { replace: true })
    } catch (err) {
      setError(err instanceof RequestError ? err.message : t('login.invalidCode'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-background via-background to-primary/5 p-4">
      <div className="absolute right-4 top-4 flex items-center gap-1">
        <LanguageToggle />
        <ThemeToggle />
      </div>
      <Card className="w-full max-w-sm shadow-lg">
        <CardHeader className="space-y-2 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center">
            {step === 'credentials' ? (
              <DisgustingLogo className="h-12 w-12" />
            ) : (
              <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary/10 text-primary">
                <KeyRound className="h-6 w-6" />
              </div>
            )}
          </div>
          <CardTitle className="text-xl">
            {step === 'credentials' ? 'Absolutely Disgusting Panel' : t('login.2faTitle')}
          </CardTitle>
          <CardDescription>
            {step === 'credentials' ? t('login.subtitle') : t('login.2faPrompt')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {step === 'credentials' ? (
            <form onSubmit={handleCredentials} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="username">{t('login.username')}</Label>
                <Input
                  id="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  autoComplete="username"
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">{t('login.password')}</Label>
                <Input
                  id="password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  autoComplete="current-password"
                  required
                />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading}>
                {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                {t('login.signIn')}
              </Button>
            </form>
          ) : (
            <form onSubmit={handleTotp} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="code">{t('login.code')}</Label>
                <Input
                  id="code"
                  inputMode="numeric"
                  autoFocus
                  placeholder="123456"
                  value={code}
                  onChange={(e) => setCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
                  className="text-center text-lg tracking-[0.5em]"
                  required
                />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <Button type="submit" className="w-full" disabled={loading || code.length !== 6}>
                {loading && <Loader2 className="h-4 w-4 animate-spin" />}
                {t('login.verify')}
              </Button>
              <Button
                type="button"
                variant="ghost"
                className="w-full"
                onClick={() => {
                  setStep('credentials')
                  setCode('')
                  setError(null)
                }}
              >
                {t('login.back')}
              </Button>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
