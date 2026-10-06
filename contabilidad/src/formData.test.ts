import { describe, expect, it } from 'vitest'
import { fieldValue } from './formData'

describe('fieldValue', () => {
  const form = new FormData()
  form.append('concepto', 'Papelería')
  form.append('adjunto', new File(['contenido'], 'nota.txt'))

  it.each([
    ['a text field', 'concepto', 'Papelería'],
    ['a field that is not in the form', 'inexistente', ''],
    ['a field that holds a file', 'adjunto', ''],
  ])('reads %s as %j', (_case, name, expected) => {
    expect(fieldValue(form, name)).toBe(expected)
  })
})
