import { useEffect, useRef } from 'react'
import { CloseIcon } from './icons'

const DISMISS_MS = 5000

export function Toast({ message, onClose }: Readonly<{ message: string; onClose: () => void }>) {
  // Keep the latest callback without restarting the timer: parents often pass a new arrow on every render.
  const close = useRef(onClose)
  useEffect(() => { close.current = onClose }, [onClose])
  useEffect(() => {
    const timeout = window.setTimeout(() => close.current(), DISMISS_MS)
    return () => window.clearTimeout(timeout)
  }, [message])

  return <div className="toast" role="status"><span>{message}</span><button className="toast-close" type="button" onClick={onClose} aria-label="Cerrar aviso"><CloseIcon /></button></div>
}
