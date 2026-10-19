/** Top-level navigation lives here so tests can observe it (jsdom cannot navigate). */
export function goTo(url: string): void {
  window.location.assign(url)
}
