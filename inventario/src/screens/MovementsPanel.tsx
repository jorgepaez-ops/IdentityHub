import { useState, type FormEvent } from 'react'
import type { Grants } from '../permissions'
import { DIRECTION_NAME, type Direction, type Entry, type Item } from '../warehouse/ledger'
import { DemoDataNote, LockedPanel, Panel } from './Panels'

interface Props {
  grants: Grants
  items: readonly Item[]
  entries: readonly Entry[]
  /** Resolves to true when the movement was recorded. */
  onPost: (sku: string, direction: Direction, quantity: number) => boolean
}

export function MovementsPanel({ grants, items, entries, onPost }: Readonly<Props>) {
  const [sku, setSku] = useState(items[0]?.sku ?? '')
  const [direction, setDirection] = useState<Direction>('in')
  const [quantity, setQuantity] = useState('')
  if (!grants.move) return <LockedPanel id="movements-heading" title="Movimientos" reason="Tu rol no permite registrar entradas ni salidas de bodega." />

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (onPost(sku, direction, Number(quantity))) setQuantity('')
  }
  const nameOf = (code: string) => items.find((item) => item.sku === code)?.name ?? code

  return (
    <Panel id="movements-heading" title="Movimientos">
      <DemoDataNote />
      <form className="card form" aria-label="Registrar movimiento" onSubmit={submit}>
        <label className="form__label" htmlFor="post-sku">Producto</label>
        <select id="post-sku" className="form__input" value={sku} onChange={(event) => setSku(event.target.value)}>
          {items.map((item) => <option key={item.sku} value={item.sku}>{`${item.sku} · ${item.name} (${item.stock})`}</option>)}
        </select>
        <label className="form__label" htmlFor="post-direction">Tipo</label>
        <select id="post-direction" className="form__input" value={direction} onChange={(event) => setDirection(event.target.value === 'out' ? 'out' : 'in')}>
          <option value="in">{DIRECTION_NAME.in}</option>
          <option value="out">{DIRECTION_NAME.out}</option>
        </select>
        <label className="form__label" htmlFor="post-quantity">Cantidad</label>
        <input id="post-quantity" className="form__input" type="number" min="1" step="1" value={quantity} required onChange={(event) => setQuantity(event.target.value)} />
        <div className="form__buttons">
          <button className="btn btn--solid" type="submit">Registrar movimiento</button>
        </div>
      </form>
      <div className="card card--flush">
        <table className="grid" aria-label="Historial de movimientos">
          <thead>
            <tr>
              <th scope="col">Folio</th>
              <th scope="col">Fecha</th>
              <th scope="col">Producto</th>
              <th scope="col">Tipo</th>
              <th scope="col" className="grid__num">Cantidad</th>
            </tr>
          </thead>
          <tbody>
            {[...entries].reverse().map((entry) => (
              <tr key={entry.folio}>
                <td className="grid__code">{entry.folio}</td>
                <td className="grid__code">{entry.day}</td>
                <td>
                  <span>{nameOf(entry.sku)}</span>
                  <span className="grid__sub">{entry.sku}</span>
                </td>
                <td><span className={`tag tag--${entry.direction}`}>{DIRECTION_NAME[entry.direction]}</span></td>
                <td className="grid__num grid__code">{entry.quantity}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Panel>
  )
}
