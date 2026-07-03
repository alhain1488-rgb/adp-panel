import { ReactElement } from 'react'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ThemeProvider } from '@/theme/theme-provider'
import { AuthProvider } from '@/auth/auth-context'
import { LanguageProvider } from '@/i18n/i18n'

export function renderWithProviders(ui: ReactElement, { route = '/' } = {}) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <LanguageProvider>
        <ThemeProvider>
          <AuthProvider>
            <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
          </AuthProvider>
        </ThemeProvider>
      </LanguageProvider>
    </QueryClientProvider>,
  )
}
