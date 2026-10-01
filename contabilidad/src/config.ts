/**
 * The Hub origin is fixed on purpose: it is also the only cross-origin entry in
 * this app's CSP (frontend/nginx/default.conf), so a different value would be
 * blocked by the browser anyway. T11 left it hardcoded in both places.
 */
export const HUB_ORIGIN = 'http://identityhub.localhost:8080'
export const ISSUER = HUB_ORIGIN
/** client_id registered in the Hub (migration 000007) and `aud` of the access token. */
export const CLIENT_ID = 'contabilidad'
export const AUDIENCE = CLIENT_ID
export const CALLBACK_PATH = '/oauth/callback'

/** The Hub requires an exact match with the registered redirect URI. */
export const redirectUri = () => `${window.location.origin}${CALLBACK_PATH}`
