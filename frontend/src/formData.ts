// Reads a text field from a submitted form; a missing field or a file yields ''.
// Avoids String(data.get(...)), which would stringify a File as "[object File]".
export function fieldValue(data: FormData, name: string): string {
  const value = data.get(name)
  return typeof value === 'string' ? value : ''
}
