import { useRef, type FormEvent } from 'react'
import type { Access } from '../access'
import { fieldValue } from '../formData'
import { KIND_LABEL, type MovementKind, type Product, type StockMovement } from '../stock'
import { DemoNotice, Locked } from './Locked'

interface Props {
  access: Access
  products: Product[]
  movements: StockMovement[]
  /** Returns true when the movement was applied, so the form can reset. */
  onRegister: (sku: string, kind: MovementKind, quantity: number) => boolean
}

export function MovementsView({ access, products, movements, onRegister }: Readonly<Props>) {
  const form = useRef<HTMLFormElement>(null)
  if (!access.canMove) return <Locked title="Movimientos" message="Tu rol no permite registrar entradas ni salidas de bodega." />

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const data = new FormData(event.currentTarget)
    const kind = fieldValue(data, 'kind')
    if (kind !== 'in' && kind !== 'out') return
    if (onRegister(fieldValue(data, 'sku'), kind, Number(fieldValue(data, 'quantity')))) form.current?.reset()
  }
  const nameOf = (sku: string) => products.find((product) => product.sku === sku)?.name ?? sku

  return (
    <section aria-labelledby="movements-title">
      <div className="view-head"><h1 id="movements-title">Movimientos</h1></div>
      <DemoNotice />
      <form className="card product-form" ref={form} onSubmit={submit} aria-label="Registrar movimiento">
        <label htmlFor="movement-sku">Producto</label>
        <select id="movement-sku" name="sku" defaultValue={products[0]?.sku}>
          {products.map((product) => <option key={product.sku} value={product.sku}>{`${product.sku} · ${product.name} (${product.stock})`}</option>)}
        </select>
        <label htmlFor="movement-kind">Tipo</label>
        <select id="movement-kind" name="kind" defaultValue="in">
          <option value="in">{KIND_LABEL.in}</option>
          <option value="out">{KIND_LABEL.out}</option>
        </select>
        <label htmlFor="movement-quantity">Cantidad</label>
        <input id="movement-quantity" name="quantity" type="number" min="1" step="1" required />
        <div className="actions"><button className="primary-button" type="submit">Registrar movimiento</button></div>
      </form>
      <div className="card table-card">
        <table aria-label="Historial de movimientos">
          <thead>
            <tr>
              <th scope="col">Folio</th>
              <th scope="col">Fecha</th>
              <th scope="col">Producto</th>
              <th scope="col">Tipo</th>
              <th scope="col" className="num">Cantidad</th>
            </tr>
          </thead>
          <tbody>
            {[...movements].reverse().map((movement) => (
              <tr key={movement.id}>
                <td><span className="mono">{movement.id}</span></td>
                <td className="mono">{movement.date}</td>
                <td>{nameOf(movement.sku)}<small>{movement.sku}</small></td>
                <td><span className={`pill pill-${movement.kind}`}>{KIND_LABEL[movement.kind]}</span></td>
                <td className="num mono">{movement.quantity}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  )
}
