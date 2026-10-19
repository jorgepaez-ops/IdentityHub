import { useState, type FormEvent } from 'react'
import type { Access } from '../access'
import { fieldValue } from '../formData'
import { LOW_STOCK_THRESHOLD, formatCop, type Product } from '../stock'
import { DemoNotice, Locked } from './Locked'

interface Props {
  access: Access
  products: Product[]
  onEdit: (sku: string, name: string, price: number) => void
}

export function CatalogView({ access, products, onEdit }: Readonly<Props>) {
  const [text, setText] = useState('')
  const [editing, setEditing] = useState<Product | null>(null)
  if (!access.canView) return <Locked title="Catálogo" message="Tu rol no incluye la consulta del catálogo de productos." />

  const needle = text.trim().toLowerCase()
  const rows = products.filter((product) => needle === '' || `${product.sku} ${product.name} ${product.category}`.toLowerCase().includes(needle))

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!editing) return
    const data = new FormData(event.currentTarget)
    const name = fieldValue(data, 'name').trim()
    const price = Number(fieldValue(data, 'price'))
    if (!name || !Number.isFinite(price) || price <= 0) return
    onEdit(editing.sku, name, price)
    setEditing(null)
  }

  return (
    <section aria-labelledby="catalog-title">
      <div className="view-head"><h1 id="catalog-title">Catálogo</h1></div>
      <DemoNotice />
      {editing && access.canEdit && (
        <form className="card product-form" key={editing.sku} onSubmit={submit} aria-label={`Editar ${editing.sku}`}>
          <label htmlFor="product-name">Nombre</label>
          <input id="product-name" name="name" defaultValue={editing.name} required maxLength={80} />
          <label htmlFor="product-price">Precio unitario (COP)</label>
          <input id="product-price" name="price" type="number" min="1" step="1" defaultValue={editing.price} required />
          <div className="actions">
            <button className="primary-button" type="submit">Guardar cambios</button>
            <button className="secondary-button" type="button" onClick={() => setEditing(null)}>Cancelar</button>
          </div>
        </form>
      )}
      <div className="card">
        <label htmlFor="catalog-search" className="field-label">Buscar producto</label>
        <input id="catalog-search" type="search" value={text} onChange={(event) => setText(event.target.value)} placeholder="SKU, nombre o categoría" />
      </div>
      <div className="card table-card">
        <table aria-label="Productos">
          <thead>
            <tr>
              <th scope="col">SKU</th>
              <th scope="col">Producto</th>
              <th scope="col" className="num">Precio</th>
              <th scope="col" className="num">Existencias</th>
              {access.canEdit && <th scope="col"><span className="sr-only">Acciones</span></th>}
            </tr>
          </thead>
          <tbody>
            {rows.map((product) => (
              <tr key={product.sku}>
                <td><span className="mono">{product.sku}</span></td>
                <td>{product.name}<small>{product.category}</small></td>
                <td className="num mono">{formatCop(product.price)}</td>
                <td className="num mono">
                  {product.stock}
                  {product.stock <= LOW_STOCK_THRESHOLD && <span className="pill pill-low">Bajo</span>}
                </td>
                {access.canEdit && (
                  <td><button className="small-button" type="button" aria-label={`Editar ${product.sku}`} onClick={() => setEditing(product)}>Editar</button></td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && <div className="empty-state"><p>Ningún producto coincide con la búsqueda.</p></div>}
      </div>
    </section>
  )
}
