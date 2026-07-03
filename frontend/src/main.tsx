import React from 'react'
import ReactDOM from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import App from './App'
import { ThemeProvider } from './theme/theme-provider'
import { AuthProvider } from './auth/auth-context'
import { LanguageProvider } from './i18n/i18n'
import { Toaster } from './components/ui/toaster'
import './index.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { staleTime: 5_000, refetchOnWindowFocus: false, retry: 1 },
  },
})

// Phase 1: no backend exists yet — the mock service worker answers the entire
// API contract. Controlled by VITE_USE_MOCKS (defaults on until Phase 7 flips it).
async function enableMocking() {
  const useMocks = import.meta.env.VITE_USE_MOCKS ?? 'true'
  if (useMocks === 'false') return
  const { worker } = await import('./mocks/browser')
  await worker.start({ onUnhandledRequest: 'bypass' })
}

void enableMocking().then(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <QueryClientProvider client={queryClient}>
        <LanguageProvider>
          <ThemeProvider>
            <AuthProvider>
              <App />
              <Toaster />
            </AuthProvider>
          </ThemeProvider>
        </LanguageProvider>
      </QueryClientProvider>
    </React.StrictMode>,
  )
})
