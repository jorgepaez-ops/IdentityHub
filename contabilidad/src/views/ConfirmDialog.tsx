import { useEffect, useId, useRef, type KeyboardEvent, type ReactNode } from 'react'

interface Props {
  title: string
  children: ReactNode
  confirmLabel: string
  tone: 'ok' | 'danger'
  onConfirm: () => void
  onCancel: () => void
}

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** Small modal confirmation: focus moves in on open, Tab stays inside, Escape cancels and focus returns to the trigger. */
export function ConfirmDialog({ title, children, confirmLabel, tone, onConfirm, onCancel }: Props) {
  const titleId = useId()
  const bodyId = useId()
  const dialog = useRef<HTMLDivElement>(null)
  const cancel = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    // Cancel is the safe default focus: pressing Enter right away never decides a movement.
    cancel.current?.focus()
    return () => trigger?.focus()
  }, [])

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Escape') {
      event.stopPropagation()
      onCancel()
      return
    }
    if (event.key !== 'Tab' || !dialog.current) return
    const items = [...dialog.current.querySelectorAll<HTMLElement>(FOCUSABLE)]
    const first = items[0]
    const last = items[items.length - 1]
    if (!first || !last) return
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }

  return (
    <div className="dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onCancel() }}>
      <div className="dialog card" ref={dialog} role="dialog" aria-modal="true" aria-labelledby={titleId} aria-describedby={bodyId} onKeyDown={onKeyDown}>
        <h2 id={titleId}>{title}</h2>
        <p id={bodyId}>{children}</p>
        <div className="dialog-actions">
          <button className="secondary-button" type="button" ref={cancel} onClick={onCancel}>Cancelar</button>
          <button className={`primary-button ${tone}`} type="button" onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
