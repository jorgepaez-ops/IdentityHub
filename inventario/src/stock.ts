// Demo data only: Inventario has no backend, so everything lives in memory for the
// length of the page. The Hub decides who may open the app; this file is the fake warehouse.

export interface Product {
  sku: string
  name: string
  category: string
  /** Unit price in Colombian pesos. */
  price: number
  stock: number
}

export type MovementKind = 'in' | 'out'

export interface StockMovement {
  id: string
  date: string
  sku: string
  kind: MovementKind
  quantity: number
}

export const KIND_LABEL: Record<MovementKind, string> = { in: 'Entrada', out: 'Salida' }

export const INITIAL_PRODUCTS: Product[] = [
  { sku: 'P-1001', name: 'Tornillo hexagonal M8 (caja x100)', category: 'Ferretería', price: 24_500, stock: 120 },
  { sku: 'P-1002', name: 'Cinta de embalaje 48 mm', category: 'Empaque', price: 6_900, stock: 340 },
  { sku: 'P-1003', name: 'Guantes de nitrilo (par)', category: 'Seguridad', price: 8_200, stock: 85 },
  { sku: 'P-1004', name: 'Estantería metálica 5 niveles', category: 'Mobiliario', price: 389_000, stock: 14 },
  { sku: 'P-1005', name: 'Etiquetas térmicas 100x50 (rollo)', category: 'Empaque', price: 17_800, stock: 9 },
  { sku: 'P-1006', name: 'Casco de seguridad blanco', category: 'Seguridad', price: 31_000, stock: 42 },
]

export const INITIAL_MOVEMENTS: StockMovement[] = [
  { id: 'MV-001', date: '2026-10-12', sku: 'P-1001', kind: 'in', quantity: 60 },
  { id: 'MV-002', date: '2026-10-14', sku: 'P-1005', kind: 'out', quantity: 15 },
  { id: 'MV-003', date: '2026-10-16', sku: 'P-1003', kind: 'in', quantity: 40 },
]

export const LOW_STOCK_THRESHOLD = 10

export const formatCop = (amount: number) => new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 }).format(amount)

export function nextMovementId(movements: readonly StockMovement[]): string {
  const highest = movements.reduce((max, movement) => Math.max(max, Number(movement.id.slice(3))), 0)
  return `MV-${String(highest + 1).padStart(3, '0')}`
}

export type MovementResult = { ok: true; products: Product[] } | { ok: false; reason: 'unknown_product' | 'invalid_quantity' | 'insufficient_stock' }

/** Applies one entry or exit; an exit can never take the stock below zero. */
export function applyMovement(products: readonly Product[], sku: string, kind: MovementKind, quantity: number): MovementResult {
  if (!Number.isInteger(quantity) || quantity <= 0) return { ok: false, reason: 'invalid_quantity' }
  const product = products.find((candidate) => candidate.sku === sku)
  if (!product) return { ok: false, reason: 'unknown_product' }
  if (kind === 'out' && quantity > product.stock) return { ok: false, reason: 'insufficient_stock' }
  const stock = kind === 'in' ? product.stock + quantity : product.stock - quantity
  return { ok: true, products: products.map((candidate) => (candidate.sku === sku ? { ...candidate, stock } : candidate)) }
}
