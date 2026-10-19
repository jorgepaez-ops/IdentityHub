// Organizations seeded by migration 000011 (RF-022, ADR 0015) with fixed identifiers.
export const organizations = {
  hub: { id: '00000000-0000-4000-8000-0000000000a1', slug: 'hub' },
  contabilidad: { id: '00000000-0000-4000-8000-0000000000a2', slug: 'contabilidad-sa' },
  empresaB: { id: '00000000-0000-4000-8000-0000000000a3', slug: 'empresa-b' },
} as const

export type Organization = (typeof organizations)[keyof typeof organizations]
