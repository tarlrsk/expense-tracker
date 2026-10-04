import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

// jsdom has no layout; the router's scroll restoration calls this.
Object.defineProperty(window, 'scrollTo', { value: vi.fn(), writable: true })
// Nor this, which the quick-entry sheet calls to bring a row into view.
Object.defineProperty(Element.prototype, 'scrollIntoView', { value: vi.fn(), writable: true })

afterEach(() => {
  cleanup()
  localStorage.clear()
  vi.unstubAllGlobals()
})
