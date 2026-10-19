import { useState, type FormEvent } from 'react'
import type { Grants } from '../permissions'
import { LOW_STOCK_AT, formatPesos, type Item } from '../warehouse/ledger'
import { DemoDataNote, LockedPanel, Panel } from './Panels'

interface Props {
  grants: Grants
  items: readonly Item[]
  onRevise: (sku: string, name: string, price: number) => void
}

function ReviseForm({ item, onSave, onCancel }: Readonly<{ item: Item; onSave: (name: string, price: number) => void; onCancel: () => void }>) {
  const [name, setName] = useState(item.name)
  const [price, setPrice] = useState(String(item.price))

  const submit = (event: FormEvent) => {
    event.preventDefault()
    const amount = Number(price)
    if (name.trim() === '' || !Number.isFinite(amount) || amount <= 0) return
    onSave(name.trim(), amount)
  }

  return (
    <form className="card form" aria-label={`Editar ${item.sku}`} onSubmit={submit}>
      <label className="form__label" htmlFor="revise-name">Nombre</label>
      <input id="revise-name" className="form__input" value={name} maxLength={80} required onChange={(event) => setName(event.target.value)} />
      <label className="form__label" htmlFor="revise-price">Precio unitario (COP)</label>
      <input id="revise-price" className="form__input" type="number" min="1" step="1" value={price} required onChange={(event) => setPrice(event.target.value)} />
      <div className="form__buttons">
        <button className="btn btn--solid" type="submit">Guardar cambios</button>
        <button className="btn btn--outline" type="button" onClick={onCancel}>Cancelar</button>
      </div>
    </form>
  )
}

export function CatalogPanel({ grants, items, onRevise }: Readonly<Props>) {
  const [query, setQuery] = useState('')
  const [editingSku, setEditingSku] = useState<string | null>(null)
  if (!grants.view) return <LockedPanel id="catalog-heading" title="Catálogo" reason="Tu rol no incluye la consulta del catálogo de productos." />

  const needle = query.trim().toLowerCase()
  const shown = items.filter((item) => `${item.sku} ${item.name} ${item.category}`.toLowerCase().includes(needle))
  const editing = grants.edit ? items.find((item) => item.sku === editingSku) : undefined

  return (
    <Panel id="catalog-heading" title="Catálogo">
      <DemoDataNote />
      {editing && (
        <ReviseForm
          key={editing.sku}
          item={editing}
          onCancel={() => setEditingSku(null)}
          onSave={(name, price) => { onRevise(editing.sku, name, price); setEditingSku(null) }}
        />
      )}
      <div className="card">
        <label className="form__label" htmlFor="catalog-query">Buscar producto</label>
        <input id="catalog-query" className="form__input" type="search" value={query} placeholder="SKU, nombre o categoría" onChange={(event) => setQuery(event.target.value)} />
      </div>
      <div className="card card--flush">
        <table className="grid" aria-label="Productos">
          <thead>
            <tr>
              <th scope="col">SKU</th>
              <th scope="col">Producto</th>
              <th scope="col" className="grid__num">Precio</th>
              <th scope="col" className="grid__num">Existencias</th>
              {grants.edit && <th scope="col"><span className="visually-hidden">Acciones</span></th>}
            </tr>
          </thead>
          <tbody>
            {shown.map((item) => (
              <tr key={item.sku}>
                <td className="grid__code">{item.sku}</td>
                <td>
                  <span>{item.name}</span>
                  <span className="grid__sub">{item.category}</span>
                </td>
                <td className="grid__num grid__code">{formatPesos(item.price)}</td>
                <td className="grid__num grid__code">
                  <span>{item.stock}</span>
                  {item.stock <= LOW_STOCK_AT && <span className="tag tag--warn">Bajo</span>}
                </td>
                {grants.edit && (
                  <td>
                    <button className="btn btn--small" type="button" aria-label={`Editar ${item.sku}`} onClick={() => setEditingSku(item.sku)}>Editar</button>
                  </td>
                )}
              </tr>
            ))}
          </tbody>
        </table>
        {shown.length === 0 && <p className="card__empty">Ningún producto coincide con la búsqueda.</p>}
      </div>
    </Panel>
  )
}
