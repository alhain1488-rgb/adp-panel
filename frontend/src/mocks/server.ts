import { setupServer } from 'msw/node'
import { handlers } from './handlers'

// Used by vitest (jsdom) so component tests exercise the same mock contract.
export const server = setupServer(...handlers)
