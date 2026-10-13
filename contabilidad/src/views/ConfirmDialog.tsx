import { useEffect, useId, useRef, type ReactNode } from 'react'

interface Props {
  title: string
  children: ReactNode
  confirmLabel: string
  tone: 'ok' | 'danger'
  onConfirm: () => void
  onCancel: () => void
  /** Where focus goes when the trigger no longer exists (a decided row loses its buttons). */
  fallbackFocus?: () => void
}

const FOCUSABLE = 'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

/** The element Tab must jump to when it would leave the dialog, or undefined when the browser may move on its own. */
function tabWrapTarget(items: HTMLElement[], current: Element | null, backwards: boolean): HTMLElement | undefined {
  if (items.length === 0) return undefined
  const edge = backwards ? 0 : items.length - 1
  return items[edge] === current ? items[items.length - 1 - edge] : undefined
}

/** Small modal confirmation: focus moves in on open, Tab stays inside, Escape cancels and focus returns to the trigger. */
export function ConfirmDialog({ title, children, confirmLabel, tone, onConfirm, onCancel, fallbackFocus }: Readonly<Props>) {
  const titleId = useId()
  const bodyId = useId()
  const backdrop = useRef<HTMLDivElement>(null)
  const dialog = useRef<HTMLDialogElement>(null)
  const cancel = useRef<HTMLButtonElement>(null)

  const fallback = useRef(fallbackFocus)
  fallback.current = fallbackFocus

  useEffect(() => {
    // Safari does not focus a clicked button, so the active element can be the body: treat that as no trigger.
    const active = document.activeElement
    const trigger = active instanceof HTMLElement && active !== document.body ? active : null
    // Cancel is the safe default focus: pressing Enter right away never decides a movement.
    cancel.current?.focus()
    return () => {
      if (trigger?.isConnected) trigger.focus()
      else fallback.current?.()
    }
  }, [])

  // The latest onCancel is kept in a ref so the listeners below attach once.
  const cancelHandler = useRef(onCancel)
  cancelHandler.current = onCancel

  // Native listeners: the dialog is a real <dialog>, so keyboard and outside-click handling attach to the DOM nodes.
  useEffect(() => {
    const dialogNode = dialog.current
    const backdropNode = backdrop.current
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.stopPropagation()
        cancelHandler.current()
        return
      }
      if (event.key !== 'Tab' || !dialogNode) return
      const items = [...dialogNode.querySelectorAll<HTMLElement>(FOCUSABLE)]
      const wrapTo = tabWrapTarget(items, document.activeElement, event.shiftKey)
      if (!wrapTo) return
      event.preventDefault()
      wrapTo.focus()
    }
    const onBackdropMouseDown = (event: MouseEvent) => {
      if (event.target === event.currentTarget) cancelHandler.current()
    }
    dialogNode?.addEventListener('keydown', onKeyDown)
    backdropNode?.addEventListener('mousedown', onBackdropMouseDown)
    return () => {
      dialogNode?.removeEventListener('keydown', onKeyDown)
      backdropNode?.removeEventListener('mousedown', onBackdropMouseDown)
    }
  }, [])

  return (
    <div className="dialog-backdrop" ref={backdrop}>
      <dialog className="dialog card" ref={dialog} open aria-modal="true" aria-labelledby={titleId} aria-describedby={bodyId}>
        <h2 id={titleId}>{title}</h2>
        <p id={bodyId}>{children}</p>
        <div className="dialog-actions">
          <button className="secondary-button" type="button" ref={cancel} onClick={onCancel}>Cancelar</button>
          <button className={`primary-button ${tone}`} type="button" onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </dialog>
    </div>
  )
}
