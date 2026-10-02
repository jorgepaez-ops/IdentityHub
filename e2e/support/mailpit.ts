import { mailpitUrl } from './config'

export interface Mail {
  id: string
  subject: string
  created: Date
  text: string
}

interface Summary {
  ID: string
  Subject: string
  Created: string
}

const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms))

async function messagesFor(to: string): Promise<Summary[]> {
  const response = await fetch(`${mailpitUrl}/api/v1/search?query=${encodeURIComponent(`to:${to}`)}`)
  if (!response.ok) throw new Error(`Mailpit search for ${to} returned ${response.status}`)
  const body = (await response.json()) as { messages?: Summary[] }
  return body.messages ?? []
}

/** Returns the Mailpit message IDs present for an address before an email-triggering action. */
export async function mailIds(to: string, timeoutMs = 20_000): Promise<Set<string>> {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    try {
      return new Set((await messagesFor(to)).map((message) => message.ID))
    } catch (error) {
      // Mailpit can briefly restart while the compose stack is becoming ready.
      if (Date.now() > deadline) throw error
    }
    if (Date.now() > deadline) throw new Error(`Mailpit IDs for ${to} were unavailable within ${timeoutMs} ms`)
    await sleep(500)
  }
}

/**
 * Waits for the newest message to `to` whose subject contains `subject`, was not already present
 * in `excludedIds`, and was created at or after `since`.
 */
export async function waitForMail(
  to: string,
  subject: string,
  since: Date,
  excludedIds: ReadonlySet<string> = new Set(),
  timeoutMs = 20_000,
): Promise<Mail> {
  const deadline = Date.now() + timeoutMs
  // Mailpit timestamps are server-side; allow a little clock skew between host and container.
  const floor = since.getTime() - 2_000
  for (;;) {
    try {
      const match = (await messagesFor(to))
        .filter((message) => !excludedIds.has(message.ID))
        .filter((message) => message.Subject.includes(subject) && new Date(message.Created).getTime() >= floor)
        .sort((a, b) => new Date(b.Created).getTime() - new Date(a.Created).getTime())[0]
      if (match) {
        const full = await fetch(`${mailpitUrl}/api/v1/message/${match.ID}`)
        if (!full.ok) throw new Error(`Mailpit message ${match.ID} returned ${full.status}`)
        const detail = (await full.json()) as { Text: string }
        return { id: match.ID, subject: match.Subject, created: new Date(match.Created), text: detail.Text }
      }
    } catch (error) {
      // Mailpit can briefly restart while the compose stack is becoming ready.
      if (Date.now() > deadline) throw error
    }
    if (Date.now() > deadline) throw new Error(`No mail "${subject}" for ${to} within ${timeoutMs} ms`)
    await sleep(500)
  }
}

export function extractCode(text: string): string {
  const match = /\b(\d{6})\b/.exec(text)
  if (!match) throw new Error('No 6-digit code in mail')
  return match[1]
}

/** Returns a six-digit value guaranteed not to equal the actual Mailpit code. */
export function incorrectCode(code: string): string {
  if (!/^\d{6}$/.test(code)) throw new Error('Expected a six-digit MFA code')
  return ((Number(code) + 1) % 1_000_000).toString().padStart(6, '0')
}

export function extractLink(text: string): string {
  const match = /https?:\/\/[^\s"<>]+/.exec(text)
  if (!match) throw new Error('No link in mail')
  return match[0]
}
