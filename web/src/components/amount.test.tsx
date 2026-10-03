import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Amount } from './amount'

function parts(container: HTMLElement) {
  const root = container.querySelector('[data-slot="amount"]')
  const [baht, satang] = Array.from(root?.children ?? [])
  return { root, baht, satang }
}

describe('Amount', () => {
  it.each([
    { value: '0.50', baht: '0', satang: '.50' },
    { value: '145.00', baht: '145', satang: '.00' },
    { value: '1234567.05', baht: '1,234,567', satang: '.05' },
    { value: '9999999999.99', baht: '9,999,999,999', satang: '.99' },
  ])('writes $value as $baht and a brass $satang', ({ value, baht, satang }) => {
    const { container } = render(<Amount value={value} />)
    const p = parts(container)

    expect(p.root).toHaveTextContent(`${baht}${satang}`)
    expect(p.root).toHaveClass('tabular-nums', 'text-foreground')
    expect(p.baht).toHaveTextContent(baht)
    expect(p.satang).toHaveTextContent(satang)
    expect(p.satang).toHaveClass('text-brass', 'text-sm')
    expect(p.baht).toHaveClass('text-base')
  })

  it('writes income in green with a plus', () => {
    const { container } = render(<Amount value="25000.00" variant="income" />)
    const p = parts(container)

    expect(p.root).toHaveTextContent('+25,000.00')
    expect(p.root).toHaveClass('text-income')
    expect(p.satang).toHaveClass('text-income')
    expect(p.satang).not.toHaveClass('text-brass')
  })

  it('uses the display sizes', () => {
    const { container } = render(<Amount value="145.00" size="display" />)
    const p = parts(container)

    expect(p.baht).toHaveClass('text-display')
    expect(p.satang).toHaveClass('text-xl')
  })

  it('shows text that is not an amount as it came', () => {
    const { container } = render(<Amount value="n/a" />)

    expect(container).toHaveTextContent('n/a')
    expect(container.querySelector('[data-slot="amount"]')).toBeNull()
  })
})
