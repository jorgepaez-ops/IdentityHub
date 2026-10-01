const PLACEHOLDER_ORIGIN = 'https://continue.invalid'
const AUTHORIZE_PATH = '/oauth/authorize'

/**
 * RF-020: after the Hub login an OAuth client sends the user back through
 * `?continue=`. Only a same-origin relative `/oauth/authorize` URL is accepted,
 * so the parameter can never become an open redirect. Anything else is ignored.
 */
export function safeContinueTarget(value: string | null): string | null {
  if (!value || !value.startsWith(AUTHORIZE_PATH)) return null
  // Backslashes, whitespace and control characters are normalized by browsers
  // into forms (`/\host`, tab/newline inside `//`) that parse as another origin.
  if (/[\\\s#]/.test(value) || [...value].some((char) => char.charCodeAt(0) < 0x20 || char.charCodeAt(0) === 0x7f)) return null
  let parsed: URL
  try {
    parsed = new URL(value, PLACEHOLDER_ORIGIN)
  } catch {
    return null
  }
  if (parsed.origin !== PLACEHOLDER_ORIGIN || parsed.pathname !== AUTHORIZE_PATH) return null
  return `${parsed.pathname}${parsed.search}`
}
