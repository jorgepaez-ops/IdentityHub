import { useEffect, useState } from 'react'
import { getHealth, getReadiness, type Readiness } from './api/client'

/**
 * Esqueleto de la semana 1.
 *
 * Su única misión es demostrar que el recorrido completo está cableado:
 * navegador → Nginx → API → PostgreSQL y RabbitMQ. Las pantallas reales de
 * autenticación (login, registro, panel de administración) llegan en la
 * semana 2, generadas contra los tipos de specs/03-api/openapi.yaml.
 */
export function App() {
  const [version, setVersion] = useState<string>('…')
  const [readiness, setReadiness] = useState<Readiness | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function probe() {
      try {
        const [health, ready] = await Promise.all([getHealth(), getReadiness()])
        if (cancelled) return
        setVersion(health.version)
        setReadiness(ready)
        setError(null)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Error desconocido')
      }
    }

    void probe()
    const timer = setInterval(probe, 5000)
    return () => {
      cancelled = true
      clearInterval(timer)
    }
  }, [])

  return (
    <main className="shell">
      <header>
        <h1>Identity Hub</h1>
        <p className="subtitle">
          Proveedor de identidad · pipeline de DevSecOps de ciclo completo
        </p>
      </header>

      <section className="card">
        <h2>Estado del sistema</h2>
        <dl>
          <dt>Versión de la API</dt>
          <dd>
            <code>{version}</code>
          </dd>
        </dl>

        {error && <p className="error">No se pudo contactar con la API: {error}</p>}

        {readiness && (
          <table>
            <thead>
              <tr>
                <th>Dependencia</th>
                <th>Estado</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(readiness.checks).map(([name, check]) => (
                <tr key={name}>
                  <td>{name}</td>
                  <td className={check.status === 'up' ? 'up' : 'down'}>
                    {check.status === 'up' ? 'operativa' : `caída — ${check.error ?? ''}`}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <footer>
        <p>
          Esqueleto de la semana 1. Este despliegue corresponde a la línea base
          deliberadamente vulnerable: <strong>no usar en producción</strong>.
        </p>
      </footer>
    </main>
  )
}
