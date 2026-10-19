import { afterEach, beforeAll, beforeEach, vi } from 'vitest'
import * as browser from '../browser'
import { FakeHub } from './fakeHub'

/**
 * Sets a test file up for the whole sign-in round trip: one Hub key pair, a clean browser storage,
 * navigation to the Hub observable (and harmless) and the address bar back at the root afterwards.
 * Returns the Hub double, valid once the tests run.
 */
export function installFakeHubEnvironment(): () => FakeHub {
  let hub: FakeHub
  beforeAll(async () => { hub = await FakeHub.create() })
  beforeEach(() => {
    sessionStorage.clear()
    localStorage.clear()
    vi.spyOn(browser, 'goTo').mockImplementation(() => undefined)
  })
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    window.history.replaceState(null, '', '/')
  })
  return () => hub
}
