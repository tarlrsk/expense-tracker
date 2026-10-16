import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { saveFile } from './download'

describe('saveFile', () => {
  const createObjectURL = vi.fn<(blob: Blob) => string>(() => 'blob:http://localhost/file-1')
  const revokeObjectURL = vi.fn<(url: string) => void>()

  beforeEach(() => {
    vi.useFakeTimers()
    createObjectURL.mockClear()
    revokeObjectURL.mockClear()
    Object.assign(URL, { createObjectURL, revokeObjectURL })
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('clicks a hidden download link for the file, removes it and frees the URL later', () => {
    const clicked: HTMLAnchorElement[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function (
      this: HTMLAnchorElement,
    ) {
      clicked.push(this)
      expect(this.isConnected).toBe(true)
    })
    const blob = new Blob(['a,b\r\n'], { type: 'text/csv' })

    saveFile(blob, 'satang-transactions.csv')

    expect(createObjectURL).toHaveBeenCalledWith(blob)
    expect(clicked).toHaveLength(1)
    const [link] = clicked
    expect(link.href).toBe('blob:http://localhost/file-1')
    expect(link.download).toBe('satang-transactions.csv')
    expect(link.isConnected).toBe(false)
    expect(revokeObjectURL).not.toHaveBeenCalled()
    vi.advanceTimersByTime(60_000)
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:http://localhost/file-1')
  })
})
