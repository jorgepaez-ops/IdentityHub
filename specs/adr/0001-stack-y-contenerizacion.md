# 0001 — Stack tecnológico y contenerización

Estado: aceptada · 2026-09-05

## Contexto

El requisito del curso pide "un pipeline de DevSecOps de ciclo completo para una
aplicación contenerizada de libre uso", con al menos front, back, base de datos,
un worker y un broker de mensajes. El presupuesto real es de un mes.

## Decisión

- **Frontend:** React + TypeScript compilado con Vite y servido por Nginx.
- **Backend:** Go. Un único módulo con dos binarios (`cmd/api`, `cmd/worker`).
- **Base de datos:** PostgreSQL.
- **Broker:** RabbitMQ. **Correo:** Mailpit como servidor SMTP de captura.
- Todo se orquesta con Docker Compose; no hay dependencias en el host más allá
  de Docker.

Go se elige por tres razones concretas para *este* proyecto: compila a un binario
estático que permite imágenes finales `scratch`/`distroless` de pocos megabytes
—lo que reduce drásticamente la superficie que Trivy tiene que escanear—, trae
`govulncheck` oficial con análisis de alcanzabilidad, y su biblioteca estándar
cubre criptografía sin dependencias de terceros.

## Consecuencias

- La superficie de CVE del backend queda casi enteramente en nuestras
  dependencias directas, no en paquetes del sistema operativo. Eso hace que los
  hallazgos de la línea base sean más didácticos: se ven y se entienden.
- Un solo módulo Go para dos binarios evita duplicar el dominio y los contratos
  de eventos, a cambio de que api y worker compartan el árbol de dependencias.
  Aceptable: son dos caras del mismo sistema.
- Nginx en el contenedor del frontend es también el punto donde viven las
  cabeceras de seguridad y el rate limiting, lo que concentra RNF-009 en un
  archivo revisable.
