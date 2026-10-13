# language: es
# Escenarios de requisitos no funcionales. A diferencia de los de RF-NNN no se derivan en un spec de
# Playwright (no llevan etiqueta @RF): cada uno nombra, en «Verificación», la prueba que lo ejecuta
# de verdad. Un escenario sin esa prueba no debe existir.
Característica: Propiedades no funcionales del sistema
  Para confiar en cómo se construye, se entrega y se opera
  Como equipo responsable del servicio
  Quiero que cada propiedad se compruebe contra los archivos y el stack reales

  # Verificación: scripts/pipeline_controls_test.py (Rnf001Containerization), job spec-drift de CI
  @RNF-001 @p0
  Escenario: Cada servicio corre en su propio contenedor y todo arranca con un solo docker compose up
    Cuando se renderiza "docker compose config" sin perfiles
    Entonces db, broker, mailpit, migrate, api, worker y web existen y ninguno exige un perfil
    Y api, worker y web construyen cada uno su propio objetivo de imagen
    Y las imágenes propias se compilan dentro de un contenedor (Dockerfile multietapa)

  # Verificación: scripts/pipeline_controls_test.py (Rnf002Pipeline), job spec-drift de CI
  @RNF-002 @p0
  Escenario: El pipeline recorre las etapas de seguridad y sus gates rompen la build
    Cuando se lee ".github/workflows/ci.yml"
    Entonces existen los jobs de pruebas, SAST, SCA, secretos, escaneo de imagen y DAST
    Y ninguno de ellos usa "continue-on-error"
    Y el escaneo de imagen sale con código 1 ante hallazgos
    Y "release.yml" genera el SBOM CycloneDX, firma, atesta y publica

  # Verificación: scripts/pipeline_controls_test.py (Rnf006Provenance), job spec-drift de CI
  @RNF-006 @p0
  Escenario: Una imagen se firma, se atesta y se verifica antes de mover latest
    Cuando se lee el job "publish" de ".github/workflows/release.yml"
    Entonces la firma Cosign precede a la atestación del SBOM, a la verificación con la identidad del workflow y a la publicación de "latest"
    Y la firma es keyless: no maneja ninguna clave privada

  # Verificación: scripts/pipeline_controls_test.py (Rnf008Hardening), job spec-drift de CI
  @RNF-008 @p0
  Escenario: Los servicios propios corren endurecidos
    Cuando se renderiza "docker compose config" y se leen los Dockerfile
    Entonces api, worker y web tienen sistema de archivos de solo lectura, cap_drop ALL y no-new-privileges
    Y la etapa final de cada imagen corre con un usuario que no es root
    Y todas las imágenes base se fijan por digest

  # Verificación: tests/load/auth-latency.js vía "make load-test", paso de carga del job e2e de CI
  @RNF-010 @p1
  Escenario: El p95 de los endpoints de autenticación se mantiene por debajo de 300 ms
    Dado que el stack está levantado y hay una sesión iniciada (el Argon2id del login queda fuera de la medición)
    Cuando se renueva la sesión, se consulta "GET /api/v1/me" y se descargan las claves JWKS bajo carga
    Entonces el p95 de cada endpoint es menor que 300 ms
    Y todas las respuestas son correctas
    Y si algún umbral se supera la ejecución termina con código distinto de cero
