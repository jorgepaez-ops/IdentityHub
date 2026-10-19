/** Where the Hub lives. It is also the only cross-origin entry in this app's CSP, so it is fixed. */
export const HUB_ORIGIN = 'http://identityhub.localhost:8080'
/** Registered in the Hub (migration 000011); it is also the expected `aud` of every access token. */
export const CLIENT_ID = 'inventario'
export const CALLBACK_PATH = '/oauth/callback'
/** The single company this app belongs to (ADR 0015); tokens carry no company claim. */
export const COMPANY = 'Empresa B'

/** The Hub compares the redirect URI byte for byte with the registered one. */
export const callbackUrl = (): string => window.location.origin + CALLBACK_PATH
