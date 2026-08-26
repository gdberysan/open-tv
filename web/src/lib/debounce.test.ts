import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { debounce } from './debounce'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

describe('debounce', () => {
  it('solo llama una vez con el último valor', () => {
    const espia = vi.fn()
    const d = debounce(espia, 300)
    d('b'); d('bb'); d('bbc')
    vi.advanceTimersByTime(299)
    expect(espia).not.toHaveBeenCalled()
    vi.advanceTimersByTime(1)
    expect(espia).toHaveBeenCalledExactlyOnceWith('bbc')
  })

  it('cancelar impide la llamada pendiente', () => {
    const espia = vi.fn()
    const d = debounce(espia, 300)
    d('x')
    d.cancelar()
    vi.advanceTimersByTime(1000)
    expect(espia).not.toHaveBeenCalled()
  })
})
