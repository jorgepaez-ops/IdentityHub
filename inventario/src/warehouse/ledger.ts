// Demo data only: Inventario has no backend, so the warehouse lives in memory for as long as the
// page does. The Hub decides who may open the app; this module is the pretend warehouse.

export interface Item {
  sku: string
  name: string
  category: string
  /** Unit price in Colombian pesos. */
  price: number
  stock: number
}

export type Direction = 'in' | 'out'

export interface Entry {
  folio: string
  day: string
  sku: string
  direction: Direction
  quantity: number
}

export interface Warehouse {
  items: Item[]
  entries: Entry[]
}

export const DIRECTION_NAME: Record<Direction, string> = { in: 'Entrada', out: 'Salida' }

/** At or below this many units an item is flagged as running low. */
export const LOW_STOCK_AT = 10

export const seedWarehouse = (): Warehouse => ({
  items: [
    { sku: 'P-1001', name: 'Tornillo hexagonal M8 (caja x100)', category: 'Ferretería', price: 24_500, stock: 120 },
    { sku: 'P-1002', name: 'Cinta de embalaje 48 mm', category: 'Empaque', price: 6_900, stock: 340 },
    { sku: 'P-1003', name: 'Guantes de nitrilo (par)', category: 'Seguridad', price: 8_200, stock: 85 },
    { sku: 'P-1004', name: 'Estantería metálica 5 niveles', category: 'Mobiliario', price: 389_000, stock: 14 },
    { sku: 'P-1005', name: 'Etiquetas térmicas 100x50 (rollo)', category: 'Empaque', price: 17_800, stock: 9 },
    { sku: 'P-1006', name: 'Casco de seguridad blanco', category: 'Seguridad', price: 31_000, stock: 42 },
  ],
  entries: [
    { folio: 'MV-001', day: '2026-10-12', sku: 'P-1001', direction: 'in', quantity: 60 },
    { folio: 'MV-002', day: '2026-10-14', sku: 'P-1005', direction: 'out', quantity: 15 },
    { folio: 'MV-003', day: '2026-10-16', sku: 'P-1003', direction: 'in', quantity: 40 },
  ],
})

const pesos = new Intl.NumberFormat('es-CO', { style: 'currency', currency: 'COP', maximumFractionDigits: 0 })
export const formatPesos = (amount: number): string => pesos.format(amount)

/** Folio that follows the highest one issued so far. */
export function nextFolio(entries: readonly Entry[]): string {
  const last = Math.max(0, ...entries.map((entry) => Number.parseInt(entry.folio.replace(/\D/g, ''), 10) || 0))
  return `MV-${String(last + 1).padStart(3, '0')}`
}

export type Problem = 'unknown_item' | 'bad_quantity' | 'not_enough_stock'
export type Posting = { ok: true; warehouse: Warehouse; entry: Entry } | { ok: false; problem: Problem }

/** Records one entry or exit. An exit can never leave the stock below zero. */
export function post(warehouse: Warehouse, sku: string, direction: Direction, quantity: number, day: string): Posting {
  if (!Number.isInteger(quantity) || quantity < 1) return { ok: false, problem: 'bad_quantity' }
  const item = warehouse.items.find((candidate) => candidate.sku === sku)
  if (!item) return { ok: false, problem: 'unknown_item' }
  const stock = item.stock + (direction === 'in' ? quantity : -quantity)
  if (stock < 0) return { ok: false, problem: 'not_enough_stock' }

  const entry: Entry = { folio: nextFolio(warehouse.entries), day, sku, direction, quantity }
  return {
    ok: true,
    entry,
    warehouse: {
      items: warehouse.items.map((candidate) => (candidate === item ? { ...candidate, stock } : candidate)),
      entries: [...warehouse.entries, entry],
    },
  }
}

/** Changes the name and price of one item. */
export const revise = (warehouse: Warehouse, sku: string, name: string, price: number): Warehouse => ({
  ...warehouse,
  items: warehouse.items.map((item) => (item.sku === sku ? { ...item, name, price } : item)),
})
