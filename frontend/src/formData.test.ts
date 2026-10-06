import { describe, expect, it } from 'vitest'
import { fieldValue } from './formData'

describe('fieldValue', () => {
  it('returns the text of a field, or an empty string when it is missing or a file', () => {
    const data = new FormData()
    data.set('name', 'Ada')
    data.set('upload', new File(['x'], 'x.txt'))
    expect(fieldValue(data, 'name')).toBe('Ada')
    expect(fieldValue(data, 'missing')).toBe('')
    expect(fieldValue(data, 'upload')).toBe('')
  })
})
