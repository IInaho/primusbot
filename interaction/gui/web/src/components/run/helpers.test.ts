import { describe, expect, it } from 'vitest'
import { compactArgs, editSummary, pathFromArgs } from './helpers'

describe('run helpers', () => {
  it('shows only the path for edit args formatted as key=value pairs', () => {
    const args = 'newString="next,value",oldString="old,value",path=/tmp/file.go,replaceAll=false'

    expect(compactArgs(args)).toBe('/tmp/file.go')
    expect(pathFromArgs(args)).toBe('/tmp/file.go')
  })

  it('keeps bash command previews readable for key=value pairs', () => {
    expect(compactArgs('command=echo hello')).toBe('echo hello')
  })

  it('summarizes edit diffs from real diff rows only', () => {
    const content = [
      '[/tmp/file.go#TAG]',
      '[write /tmp/file.go]',
      'metadata: should not count',
      '-1:old',
      '+1:next',
      '---',
      '+notaline: ignored',
      '-x: ignored',
    ].join('\n')

    expect(editSummary(content)).toBe('+1 −1')
  })
})
