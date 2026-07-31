import { describe, expect, it } from 'vitest'
import {
  isWailsEnvironment,
  safeAbort,
  safeEventsOn,
  safeCurrentModel,
  safeSendMessage,
} from '../wails'

describe('isWailsEnvironment', () => {
  it('returns false when window.go is undefined', () => {
    expect(isWailsEnvironment()).toBe(false)
  })
})

describe('safe wrappers', () => {
  it('return no-ops when Wails is unavailable', async () => {
    expect(safeEventsOn('test', () => {})).toBeInstanceOf(Function)
    expect(safeAbort()).toBeUndefined()
    await expect(safeCurrentModel()).resolves.toBe('')
    await expect(safeSendMessage('hello')).resolves.toBeUndefined()
  })
})
