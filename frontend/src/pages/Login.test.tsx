import { describe, expect, it, beforeEach } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders } from '@/test/utils'
import LoginPage from './Login'

beforeEach(() => {
  localStorage.clear()
  // The UI defaults to Russian; pin English so these label assertions are stable.
  localStorage.setItem('adp_lang', 'en')
})

describe('LoginPage', () => {
  it('advances to the TOTP step after valid credentials', async () => {
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />)

    await user.clear(screen.getByLabelText(/username/i))
    await user.type(screen.getByLabelText(/username/i), 'admin')
    await user.type(screen.getByLabelText(/password/i), 'secret')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() =>
      expect(screen.getByText(/two-factor authentication/i)).toBeInTheDocument(),
    )
  })

  it('shows an error on wrong password', async () => {
    const user = userEvent.setup()
    renderWithProviders(<LoginPage />)

    await user.type(screen.getByLabelText(/password/i), 'wrong')
    await user.click(screen.getByRole('button', { name: /sign in/i }))

    await waitFor(() =>
      expect(screen.getByText(/invalid username or password/i)).toBeInTheDocument(),
    )
  })
})
