// Reads a text field from a submitted form; a missing field or a file yields ''.
export function fieldValue(data: FormData, name: string): string {
  const value = data.get(name)
  return typeof value === 'string' ? value : ''
}
