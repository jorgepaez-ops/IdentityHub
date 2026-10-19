/** Top-level navigation, isolated so tests can observe it (jsdom cannot navigate). */
export function redirectTo(url: string): void {
  window.location.assign(url)
}
