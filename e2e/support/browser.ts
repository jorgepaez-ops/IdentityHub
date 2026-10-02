import { expect, type Page } from '@playwright/test'
import { extractCode, mailIds, waitForMail } from './mailpit'

export interface BrowserMfaOptions {
  /** Mailpit IDs captured before the login was triggered; taken now when omitted. */
  existingMailIds?: ReadonlySet<string>
  /** Lower bound for the code email's timestamp; taken now when omitted. */
  since?: Date
}

/**
 * Fills the Hub login form on the current page, reads the emailed MFA code from Mailpit and submits
 * it, the way a person would. Callers that triggered earlier mail pass their own snapshot.
 */
export async function browserLoginWithMfa(
  page: Page,
  email: string,
  password: string,
  options: BrowserMfaOptions = {},
): Promise<void> {
  const existingMailIds = options.existingMailIds ?? (await mailIds(email))
  const since = options.since ?? new Date()
  await page.locator('#email').fill(email)
  await page.locator('#password').fill(password)
  await page.getByRole('button', { name: 'Iniciar sesión' }).click()
  await expect(page.locator('#mfa-code')).toBeVisible()
  const mail = await waitForMail(email, 'sign-in code', since, existingMailIds)
  await page.locator('#mfa-code').fill(extractCode(mail.text))
  await page.getByRole('button', { name: 'Verificar' }).click()
}
